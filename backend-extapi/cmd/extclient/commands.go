package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/cimb-niaga/cms/backend-extapi/internal/audit"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/cimb-niaga/cms/backend-extapi/internal/repository"
	"github.com/cimb-niaga/cms/pkg/userrules"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	auditIP          = "cli"
	entityAPIClient  = "api_client"
	entityUser       = "user"
	maxNameLen       = 100
	serviceUserEmail = "@service.crown.invalid" // .invalid can never receive mail
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// grants is the audited view of a client's rights: never a secret or hash.
type grants struct {
	Scopes          []string       `json:"scopes"`
	AssignableRoles []string       `json:"assignable_roles"`
	AllowedCIDRs    []netip.Prefix `json:"allowed_cidrs"`
}

// ---- input validation (before any DB access) ----

func splitList(s string) []string {
	out := []string{}
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

func parseScopes(s string) ([]string, error) {
	scopes, err := extauth.ParseScopes(s)
	if err != nil {
		return nil, fmt.Errorf("%w: --scopes: %v", errUsage, err)
	}
	if len(scopes) == 0 {
		return nil, fmt.Errorf("%w: --scopes needs at least one scope", errUsage)
	}
	return scopes, nil
}

// parseRoles rejects privileged roles here; existence is checked in the tx.
func parseRoles(s string) ([]string, error) {
	roles := splitList(s)
	for _, r := range roles {
		if userrules.IsPrivileged(r) {
			return nil, fmt.Errorf("%w: --roles: %q is a privileged role and can never be assigned", errUsage, r)
		}
	}
	return roles, nil
}

func parseCIDRs(s string) ([]netip.Prefix, error) {
	out := []netip.Prefix{}
	for _, f := range splitList(s) {
		p, err := netip.ParsePrefix(f)
		if err != nil {
			return nil, fmt.Errorf("%w: --cidrs: %q is not a CIDR", errUsage, f)
		}
		if p.Bits() == 0 {
			return nil, fmt.Errorf("%w: --cidrs: %q allows every address; /0 is not allowed", errUsage, f)
		}
		if p != p.Masked() {
			return nil, fmt.Errorf("%w: --cidrs: %q has host bits set; use %s", errUsage, f, p.Masked())
		}
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out, nil
}

func parseClientID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !extauth.ValidClientIDFormat(s) {
		return "", fmt.Errorf("%w: --client-id: not a valid client id", errUsage)
	}
	return s, nil
}

// slug turns a display name into the [a-z0-9-] part of svc-<slug>.
func slug(name string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func warnEmptyCIDRs(c *cli, cidrs []netip.Prefix) {
	if len(cidrs) == 0 {
		_, _ = fmt.Fprintln(c.stderr, "WARNING: --cidrs is empty: this client cannot get a token or call the API until CIDRs are set (set-grants --cidrs).")
	}
}

// ---- in-transaction checks ----

// actorID resolves --actor to an active APPACCESS user (Req 10.4 audit actor).
func actorID(ctx context.Context, q *repository.Queries, username string) (int64, error) {
	row, err := q.GetActiveAppAccessUser(ctx, strings.TrimSpace(username))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("--actor %q is not an active APPACCESS user", username)
	}
	if err != nil {
		return 0, fmt.Errorf("look up actor: %w", err)
	}
	return row.ID, nil
}

// checkRolesExist requires every role to be a row in roles (Req 9.2).
func checkRolesExist(ctx context.Context, q *repository.Queries, roles []string) error {
	for _, r := range roles {
		_, err := q.GetRoleIDByName(ctx, r)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("--roles: role %q does not exist", r)
		}
		if err != nil {
			return fmt.Errorf("look up role %q: %w", r, err)
		}
	}
	return nil
}

func getClient(ctx context.Context, q *repository.Queries, clientID string) (repository.GetApiClientByClientIDRow, error) {
	row, err := q.GetApiClientByClientID(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, fmt.Errorf("client %s not found", clientID)
	}
	if err != nil {
		return row, fmt.Errorf("get client: %w", err)
	}
	return row, nil
}

// conflict turns a unique violation into a readable error.
func conflict(err error, what string) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == userrules.PgUniqueViolation {
		return fmt.Errorf("%s already exists (%s)", what, pg.ConstraintName)
	}
	return err
}

// ---- commands ----

func cmdCreate(ctx context.Context, c *cli, args []string) error {
	fs := flagSet("create", c.stderr)
	name := fs.String("name", "", "display name (1-100 chars, unique)")
	scopesF := fs.String("scopes", "", "scopes, e.g. users:read,users:write")
	rolesF := fs.String("roles", "", "roles the client may assign (non-privileged)")
	cidrsF := fs.String("cidrs", "", "allowed source CIDRs, e.g. 10.0.0.10/32")
	actor := fs.String("actor", "", "username of an active APPACCESS user")
	if err := parse(fs, args, "name", "scopes", "actor"); err != nil {
		return err
	}
	n := strings.TrimSpace(*name)
	sl := slug(n)
	if len(n) > maxNameLen || sl == "" {
		return fmt.Errorf("%w: --name must be 1-%d chars with at least one letter or digit", errUsage, maxNameLen)
	}
	scopes, err := parseScopes(*scopesF)
	if err != nil {
		return err
	}
	roles, err := parseRoles(*rolesF)
	if err != nil {
		return err
	}
	cidrs, err := parseCIDRs(*cidrsF)
	if err != nil {
		return err
	}

	clientID, err := extauth.NewClientID()
	if err != nil {
		return err
	}
	secret, err := extauth.NewSecret()
	if err != nil {
		return err
	}
	hash, err := extauth.HashSecret(secret)
	if err != nil {
		return err
	}

	svc := repository.CreateServiceUserParams{
		Username: "svc-" + sl,
		FullName: "Service: " + n,
		Email:    "svc-" + sl + serviceUserEmail,
	}
	aw := audit.NewWriter()
	err = c.inTx(ctx, func(tx repository.DBTX) error {
		q := repository.New(tx)
		actorUID, err := actorID(ctx, q, *actor)
		if err != nil {
			return err
		}
		if err := checkRolesExist(ctx, q, roles); err != nil {
			return err
		}
		svcID, err := q.CreateServiceUser(ctx, svc)
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("role EXT-CLIENT is missing (migration 039 not applied?)")
		}
		if err != nil {
			return conflict(err, "service account "+svc.Username)
		}
		row, err := q.CreateApiClient(ctx, repository.CreateApiClientParams{
			ClientID: clientID, Name: n, SecretHash: hash, Scopes: scopes,
			AssignableRoles: roles, AllowedCidrs: cidrs, ServiceUserID: svcID, CreatedBy: actorUID,
		})
		if err != nil {
			return conflict(err, "client "+n)
		}
		if err := aw.Write(ctx, tx, audit.Entry{
			ActorID: actorUID, Action: "user_created", EntityType: entityUser, EntityID: svcID, IP: auditIP,
			After: map[string]any{
				"username": svc.Username, "full_name": svc.FullName, "email": svc.Email,
				"auth_source": userrules.AuthSourceService, "role": userrules.ServiceRole,
				"is_active": false, "is_karyawan": false,
			},
		}); err != nil {
			return err
		}
		return aw.Write(ctx, tx, audit.Entry{
			ActorID: actorUID, Action: "api_client_created", EntityType: entityAPIClient, EntityID: row.ID, IP: auditIP,
			After: map[string]any{
				"client_id": row.ClientID, "name": row.Name, "credential_version": row.CredentialVersion,
				"scopes": row.Scopes, "assignable_roles": row.AssignableRoles, "allowed_cidrs": row.AllowedCidrs,
				"service_user_id": row.ServiceUserID, "is_active": row.IsActive,
			},
		})
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.stdout, "client_id:        %s\nclient_secret:    %s\nservice_username: %s\n", clientID, secret, svc.Username)
	printSecretReminder(c)
	warnEmptyCIDRs(c, cidrs)
	return nil
}

func printSecretReminder(c *cli) {
	_, _ = fmt.Fprintln(c.stderr, "The secret is shown only this once and cannot be recovered. Hand it over through a secure channel; do not paste it into tickets, chat, or logs.")
}

func cmdRotate(ctx context.Context, c *cli, args []string) error {
	fs := flagSet("rotate", c.stderr)
	clientF := fs.String("client-id", "", "client id")
	actor := fs.String("actor", "", "username of an active APPACCESS user")
	if err := parse(fs, args, "client-id", "actor"); err != nil {
		return err
	}
	clientID, err := parseClientID(*clientF)
	if err != nil {
		return err
	}
	secret, err := extauth.NewSecret()
	if err != nil {
		return err
	}
	hash, err := extauth.HashSecret(secret)
	if err != nil {
		return err
	}
	var version int32
	err = c.inTx(ctx, func(tx repository.DBTX) error {
		q := repository.New(tx)
		actorUID, err := actorID(ctx, q, *actor)
		if err != nil {
			return err
		}
		row, err := q.RotateApiClientSecret(ctx, repository.RotateApiClientSecretParams{SecretHash: hash, ClientID: clientID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("client %s not found", clientID)
		}
		if err != nil {
			return fmt.Errorf("rotate secret: %w", err)
		}
		version = row.CredentialVersion
		return audit.NewWriter().Write(ctx, tx, audit.Entry{
			ActorID: actorUID, Action: "api_client_secret_rotated", EntityType: entityAPIClient, EntityID: row.ID, IP: auditIP,
			Before: map[string]any{"client_id": row.ClientID, "credential_version": row.CredentialVersion - 1},
			After:  map[string]any{"client_id": row.ClientID, "credential_version": row.CredentialVersion},
		})
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.stdout, "client_id:          %s\nclient_secret:      %s\ncredential_version: %d\n", clientID, secret, version)
	printSecretReminder(c)
	_, _ = fmt.Fprintln(c.stderr, "The old secret and every token issued before this rotation are rejected from now on.")
	return nil
}

func cmdSetActive(ctx context.Context, c *cli, args []string, active bool) error {
	name, action := "disable", "api_client_disabled"
	if active {
		name, action = "enable", "api_client_enabled"
	}
	fs := flagSet(name, c.stderr)
	clientF := fs.String("client-id", "", "client id")
	actor := fs.String("actor", "", "username of an active APPACCESS user")
	if err := parse(fs, args, "client-id", "actor"); err != nil {
		return err
	}
	clientID, err := parseClientID(*clientF)
	if err != nil {
		return err
	}
	changed := false
	err = c.inTx(ctx, func(tx repository.DBTX) error {
		q := repository.New(tx)
		actorUID, err := actorID(ctx, q, *actor)
		if err != nil {
			return err
		}
		before, err := getClient(ctx, q, clientID)
		if err != nil {
			return err
		}
		if before.IsActive == active {
			return nil // already in that state: no write, no audit
		}
		row, err := q.SetApiClientActive(ctx, repository.SetApiClientActiveParams{IsActive: active, ClientID: clientID})
		if err != nil {
			return fmt.Errorf("%s client: %w", name, err)
		}
		changed = true
		return audit.NewWriter().Write(ctx, tx, audit.Entry{
			ActorID: actorUID, Action: action, EntityType: entityAPIClient, EntityID: row.ID, IP: auditIP,
			Before: map[string]any{"client_id": clientID, "is_active": before.IsActive},
			After:  map[string]any{"client_id": clientID, "is_active": row.IsActive},
		})
	})
	if err != nil {
		return err
	}
	if !changed {
		_, _ = fmt.Fprintf(c.stdout, "client %s is already %sd; nothing changed\n", clientID, name)
		return nil
	}
	_, _ = fmt.Fprintf(c.stdout, "client %s %sd\n", clientID, name)
	return nil
}

func cmdSetGrants(ctx context.Context, c *cli, args []string) error {
	fs := flagSet("set-grants", c.stderr)
	clientF := fs.String("client-id", "", "client id")
	scopesF := fs.String("scopes", "", "new scopes (replaces the list)")
	rolesF := fs.String("roles", "", "new assignable roles (replaces the list; empty clears it)")
	cidrsF := fs.String("cidrs", "", "new allowed CIDRs (replaces the list; empty clears it)")
	actor := fs.String("actor", "", "username of an active APPACCESS user")
	if err := parse(fs, args, "client-id", "actor"); err != nil {
		return err
	}
	clientID, err := parseClientID(*clientF)
	if err != nil {
		return err
	}
	got := passed(fs)
	if !got["scopes"] && !got["roles"] && !got["cidrs"] {
		return fmt.Errorf("%w: give at least one of --scopes, --roles, --cidrs", errUsage)
	}
	var newScopes, newRoles []string
	var newCIDRs []netip.Prefix
	if got["scopes"] {
		if newScopes, err = parseScopes(*scopesF); err != nil {
			return err
		}
	}
	if got["roles"] {
		if newRoles, err = parseRoles(*rolesF); err != nil {
			return err
		}
	}
	if got["cidrs"] {
		if newCIDRs, err = parseCIDRs(*cidrsF); err != nil {
			return err
		}
	}

	var after grants
	err = c.inTx(ctx, func(tx repository.DBTX) error {
		q := repository.New(tx)
		actorUID, err := actorID(ctx, q, *actor)
		if err != nil {
			return err
		}
		cur, err := getClient(ctx, q, clientID)
		if err != nil {
			return err
		}
		before := grants{Scopes: cur.Scopes, AssignableRoles: cur.AssignableRoles, AllowedCIDRs: cur.AllowedCidrs}
		after = before
		if got["scopes"] {
			after.Scopes = newScopes
		}
		if got["roles"] {
			if err := checkRolesExist(ctx, q, newRoles); err != nil {
				return err
			}
			after.AssignableRoles = newRoles
		}
		if got["cidrs"] {
			after.AllowedCIDRs = newCIDRs
		}
		row, err := q.UpdateApiClientGrants(ctx, repository.UpdateApiClientGrantsParams{
			Scopes: after.Scopes, AssignableRoles: after.AssignableRoles, AllowedCidrs: after.AllowedCIDRs, ClientID: clientID,
		})
		if err != nil {
			return fmt.Errorf("update grants: %w", err)
		}
		return audit.NewWriter().Write(ctx, tx, audit.Entry{
			ActorID: actorUID, Action: "api_client_grants_updated", EntityType: entityAPIClient, EntityID: row.ID, IP: auditIP,
			Before: before, After: after,
		})
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.stdout, "client %s grants updated: scopes=%s roles=%s cidrs=%s\n",
		clientID, join(after.Scopes), join(after.AssignableRoles), joinPrefixes(after.AllowedCIDRs))
	warnEmptyCIDRs(c, after.AllowedCIDRs)
	return nil
}

func cmdList(ctx context.Context, c *cli, args []string) error {
	fs := flagSet("list", c.stderr)
	if err := parse(fs, args); err != nil {
		return err
	}
	pool, err := c.db(ctx)
	if err != nil {
		return err
	}
	rows, err := repository.New(pool).ListApiClients(ctx)
	if err != nil {
		return fmt.Errorf("list clients: %w", err)
	}
	w := tabwriter.NewWriter(c.stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "CLIENT_ID\tNAME\tACTIVE\tVERSION\tSCOPES\tROLES\tCIDRS\tSERVICE_USER_ID\tSERVICE_USERNAME")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%t\t%d\t%s\t%s\t%s\t%d\t%s\n",
			r.ClientID, r.Name, r.IsActive, r.CredentialVersion, join(r.Scopes), join(r.AssignableRoles),
			joinPrefixes(r.AllowedCidrs), r.ServiceUserID, r.ServiceUsername)
	}
	return w.Flush()
}

func join(s []string) string {
	if len(s) == 0 {
		return "-"
	}
	return strings.Join(s, ",")
}

func joinPrefixes(ps []netip.Prefix) string {
	s := make([]string, len(ps))
	for i, p := range ps {
		s[i] = p.String()
	}
	return join(s)
}
