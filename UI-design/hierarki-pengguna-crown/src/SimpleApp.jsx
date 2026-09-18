import { useState } from "react";
import { Button, Input, Select, SelectTrigger, SelectValue, SelectContent, SelectGroup, SelectItem, Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter, FieldGroup, Field, FieldLabel, FieldDescription, Badge } from "@kits/shadcn-ui";
import { Search, Pencil, ChevronRight, Users, ArrowRight, Check, Network, ShieldCheck, User, Wrench } from "lucide-react";
import { Toaster, toast } from "sonner";
import { USERS, domainOf, levelOf, DOMAINS } from "./data";
import "./simple.css";

const initials = (name) => name.split(" ").slice(0, 2).map(s => s[0]).join("");
const role = (u) => u.role.toLowerCase().replace(/_/g, ".");
function Choice({ value, onChange, label, children, id }) {
  return <Select value={value} onValueChange={onChange}>
    <SelectTrigger id={id} aria-label={label}><SelectValue /></SelectTrigger>
    <SelectContent><SelectGroup>{children}</SelectGroup></SelectContent>
  </Select>;
}
function Person({ u, changed }) {
  return <div className="person"><span className="avatar">{initials(u.name)}</span><div><strong>{u.name}</strong><span className="username">{u.username}</span></div>{changed && <Check size={16} className="saved" aria-label="Diubah dalam prototipe"/>}</div>;
}
export default function App() {
  const [users, setUsers] = useState(USERS);
  const [query, setQuery] = useState("");
  const [domain, setDomain] = useState("atm");
  const [editing, setEditing] = useState(null);
  const [supervisor, setSupervisor] = useState("none");
  const [tier, setTier] = useState("1");
  const [changed, setChanged] = useState([]);
  const nameOf = id => users.find(u => u.id === id)?.name;
  const visible = users.filter(u => (domain === "all" || domainOf(u) === domain)
    && `${u.name} ${u.username} ${role(u)} ${nameOf(u.supervisorId) || ""}`.toLowerCase().includes(query.toLowerCase()));
  const spvs = visible.filter(u => levelOf(u) === "checker");
  const makers = visible.filter(u => levelOf(u) === "maker");
  const admins = visible.filter(u => levelOf(u) === "admin");
  const teamSize = id => users.filter(u => u.supervisorId === id).length;
  const maker = editing && levelOf(editing) === "maker";
  const candidates = editing ? users.filter(u => levelOf(u) === "checker"
    && domainOf(u) === domainOf(editing) && u.id !== editing.id
    && (domainOf(u) !== "vendor" || u.vendor === editing.vendor)) : [];
  const openEdit = (u) => {
    setEditing(u); setSupervisor(String(u.supervisorId || "none")); setTier(String(u.tier || 1));
  };
  const dirty = editing && (maker ? supervisor !== String(editing.supervisorId || "none") : tier !== String(editing.tier));
  const save = () => {
    setUsers(prev => prev.map(u => u.id === editing.id ? {...u, ...(maker
      ? {supervisorId: supervisor === "none" ? null : Number(supervisor)}
      : {tier: Number(tier)})} : u));
    setChanged(prev => [...new Set([...prev, editing.id])]);
    toast.success(`Pengaturan ${editing.name} disimpan di prototipe.`);
    setEditing(null);
  };
  const reset = () => { setQuery(""); setDomain("all"); };
  return <div>
    <header className="topbar">
      <div className="identity"><span className="brand-icon"><Network size={22}/></span><strong>CROWN</strong><span className="product">Cash Management System</span></div>
      <Badge variant="secondary">Prototipe interaktif</Badge>
    </header>
    <main>
      <div className="breadcrumb">Pengaturan <ChevronRight size={15}/> <span>Hierarki pengguna</span></div>
      <div className="page-heading"><div><h1>Hierarki pengguna</h1><p>Supervisor dan pengguna biasa dipisah. Atur level persetujuan di satu tabel, atasan di tabel lainnya.</p></div><div className="page-icon"><Users size={28}/></div></div>

      <div className="toolbar" role="search">
        <div className="search-field"><label htmlFor="search">Cari pengguna</label><div className="search-input"><Search size={18}/><Input id="search" placeholder="Nama, peran, atau atasan" value={query} onChange={e => setQuery(e.target.value)}/></div></div>
        <div className="domain-field"><label htmlFor="domain">Domain</label><Choice id="domain" label="Domain" value={domain} onChange={setDomain}>
          <SelectItem value="all">Semua domain</SelectItem>
          {Object.entries(DOMAINS).filter(([k]) => k !== "all").map(([k,v]) => <SelectItem key={k} value={k}>{v.label}</SelectItem>)}
        </Choice></div>
        <span className="result-count">{spvs.length} supervisor <i>·</i> {makers.length} pengguna{admins.length ? <> <i>·</i> {admins.length} admin</> : null}</span>
      </div>

      {!visible.length ? <div className="register"><div className="empty"><Search size={25}/><strong>Pengguna tidak ditemukan</strong><p>Coba nama lain atau pilih semua domain.</p><Button variant="outline" onClick={reset}>Reset pencarian</Button></div></div> : <>

        <section className="register spv-register" aria-labelledby="spv-title">
          <div className="section-head">
            <span className="section-icon spv"><ShieldCheck size={19}/></span>
            <div><h2 id="spv-title">Supervisor (SPV)</h2><p>Menyetujui pengajuan. Yang bisa diatur: level persetujuan.</p></div>
            <span className="section-count">{spvs.length}</span>
          </div>
          {spvs.length ? <div className="table-scroll">
            <table><thead><tr><th>Supervisor</th><th>Peran</th><th>Level persetujuan</th><th>Anggota</th><th><span className="sr-only">Aksi</span></th></tr></thead>
            <tbody>{spvs.map(u => <tr key={u.id}>
              <td><Person u={u} changed={changed.includes(u.id)}/></td>
              <td><code className="role checker">{role(u)}</code></td>
              <td><span className="level">Level {u.tier}</span></td>
              <td>{teamSize(u.id) ? <span className="team-size">{teamSize(u.id)} pengguna</span> : <span className="secondary">Belum ada</span>}</td>
              <td><Button variant="ghost" onClick={() => openEdit(u)} aria-label={`Ubah ${u.name}`}><Pencil/>Ubah</Button></td>
            </tr>)}</tbody></table>
          </div> : <p className="section-empty">Tidak ada supervisor pada filter ini.</p>}
        </section>

        <section className="register" aria-labelledby="user-title">
          <div className="section-head">
            <span className="section-icon"><User size={19}/></span>
            <div><h2 id="user-title">Pengguna biasa</h2><p>Membuat pengajuan. Yang bisa diatur: atasan.</p></div>
            <span className="section-count">{makers.length}</span>
          </div>
          {makers.length ? <div className="table-scroll">
            <table><thead><tr><th>Pengguna</th><th>Peran</th><th>Atasan</th><th><span className="sr-only">Aksi</span></th></tr></thead>
            <tbody>{makers.map(u => <tr key={u.id}>
              <td><Person u={u} changed={changed.includes(u.id)}/></td>
              <td><code className="role">{role(u)}</code></td>
              <td>{u.supervisorId ? <div className="supervisor"><span className="mini-avatar">{initials(nameOf(u.supervisorId))}</span>{nameOf(u.supervisorId)}</div>
                : <span className="unassigned">Belum diatur</span>}</td>
              <td><Button variant="ghost" onClick={() => openEdit(u)} aria-label={`Ubah ${u.name}`}><Pencil/>Ubah</Button></td>
            </tr>)}</tbody></table>
          </div> : <p className="section-empty">Tidak ada pengguna biasa pada filter ini.</p>}
          <footer className="table-footer"><span><Users size={16}/> Satu pengguna, satu atasan dalam domain yang sama.</span><span>Data contoh</span></footer>
        </section>

        {admins.length ? <section className="register" aria-labelledby="admin-title">
          <div className="section-head">
            <span className="section-icon"><Wrench size={19}/></span>
            <div><h2 id="admin-title">Admin &amp; support</h2><p>Di luar rantai persetujuan, tidak punya atasan.</p></div>
            <span className="section-count">{admins.length}</span>
          </div>
          <div className="table-scroll">
            <table><thead><tr><th>Pengguna</th><th>Peran</th><th>Keterangan</th></tr></thead>
            <tbody>{admins.map(u => <tr key={u.id}>
              <td><Person u={u} changed={false}/></td>
              <td><code className="role">{role(u)}</code></td>
              <td><span className="secondary">Tidak ikut maker-checker</span></td>
            </tr>)}</tbody></table>
          </div>
        </section> : null}
      </>}

      <p className="demo-note">Perubahan hanya untuk simulasi. Tidak mengubah pengguna atau persetujuan di CROWN.</p>
    </main>
    <Dialog open={!!editing} onOpenChange={open => !open && setEditing(null)}>
      <DialogContent className="editor">
        <DialogHeader><DialogTitle>{maker ? "Ubah atasan" : "Ubah level persetujuan"}</DialogTitle><DialogDescription>{maker ? "Pilih supervisor yang menyetujui pengajuan pengguna ini." : "Atur sampai level mana supervisor ini boleh menyetujui."}</DialogDescription></DialogHeader>
        {editing && <>
          <div className="editing-person"><span className="avatar">{initials(editing.name)}</span><div><strong>{editing.name}</strong><code>{role(editing)}</code></div><span className={maker ? "role-tag" : "role-tag spv"}>{maker ? "Pengguna biasa" : "Supervisor"}</span></div>
          <FieldGroup>
            {maker ? <Field><FieldLabel htmlFor="supervisor">Atasan</FieldLabel><Choice id="supervisor" label="Atasan" value={supervisor} onChange={setSupervisor}>
              <SelectItem value="none">Belum diatur</SelectItem>{candidates.map(u => <SelectItem key={u.id} value={String(u.id)}>{u.name}</SelectItem>)}
            </Choice><FieldDescription>{candidates.length ? "Pilih supervisor dalam domain yang sama." : "Belum ada supervisor yang sesuai untuk pengguna ini."}</FieldDescription></Field>
            : <Field><FieldLabel htmlFor="tier">Level persetujuan</FieldLabel><Choice id="tier" label="Level persetujuan" value={tier} onChange={setTier}><SelectItem value="1">Level 1</SelectItem><SelectItem value="2">Level 2</SelectItem></Choice><FieldDescription>Level contoh untuk desain, tanpa asumsi plafon nominal.</FieldDescription></Field>}
          </FieldGroup>
          {dirty && <div className="change-preview"><span>{maker ? nameOf(editing.supervisorId) || "Belum diatur" : `Level ${editing.tier}`}</span><ArrowRight size={16}/><strong>{maker ? nameOf(Number(supervisor)) || "Belum diatur" : `Level ${tier}`}</strong></div>}
          <DialogFooter><Button variant="outline" onClick={() => setEditing(null)}>Batal</Button><Button disabled={!dirty} onClick={save}>Simpan perubahan</Button></DialogFooter>
        </>}
      </DialogContent>
    </Dialog>
    <Toaster position="bottom-right" theme="light"/>
  </div>;
}
