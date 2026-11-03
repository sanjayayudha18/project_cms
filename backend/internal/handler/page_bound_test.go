package handler

import (
	"net/url"
	"strconv"
	"testing"
)

// page feeds int4 SQL math ((page-1)*page_size) and int32() conversions;
// anything above maxPage must be a 400, never a wrapped negative OFFSET (500).
func TestPageParsers_RejectAboveMaxPage(t *testing.T) {
	over := url.Values{"page": {strconv.Itoa(maxPage + 1)}}
	huge := url.Values{"page": {"3000000000"}}
	atMax := url.Values{"page": {strconv.Itoa(maxPage)}}

	for _, q := range []url.Values{over, huge} {
		if _, err := parsePageParam(q, 1); err == nil {
			t.Errorf("parsePageParam(%v): want error", q)
		}
		if _, _, err := parsePageParams(q); err == nil {
			t.Errorf("parsePageParams(%v): want error", q)
		}
		if _, _, err := parseRegionPageParams(q); err == nil {
			t.Errorf("parseRegionPageParams(%v): want error", q)
		}
	}

	if p, err := parsePageParam(atMax, 1); err != nil || p != maxPage {
		t.Errorf("parsePageParam at max: got %d, %v", p, err)
	}
	if p, _, err := parsePageParams(atMax); err != nil || p != maxPage {
		t.Errorf("parsePageParams at max: got %d, %v", p, err)
	}
	if p, err := parsePageParam(url.Values{}, 7); err != nil || p != 7 {
		t.Errorf("parsePageParam default: got %d, %v", p, err)
	}
}
