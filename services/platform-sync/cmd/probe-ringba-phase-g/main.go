// Ringba write-back Phase G — convert loop on a CLEAN sandbox call.
// Research only. Writes: conversion webhook fire (+ optional create/delete of CALYX-SANDBOX-PHASE-G).
// Never payments/void/annotate. Never prints tokens or ANI.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sandboxCallID   = "RGB0B96293B9F8915CA75C5CCDCF7BCFD0CFAAEDB9CV3NYI01"
	sandboxCamp     = "Calyx-Sandbox-DO-NOT-USE"
	sandboxTargetID = "TAd5332f67bf994754b8a23b8847feb413"
	integrationID   = "ce6fd338-af48-47d8-850a-bd10b855e2f4"
	tenantID        = "11111111-1111-1111-1111-111111111111"
)

func main() {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := envOr("DATABASE_URL", "postgres://callloom:callloom_dev@postgres:5432/callloom?sslmode=disable")
	if key == "" {
		fatal("CREDENTIAL_ENCRYPTION_KEY required")
	}
	callID := envOr("SANDBOX_CALL_ID", sandboxCallID)
	integ := envOr("INTEGRATION_ID", integrationID)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	must(err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `SELECT set_config('app.current_tenant', $1, false)`, tenantID)
	must(err)

	var blob []byte
	must(pool.QueryRow(ctx, `SELECT api_credentials_blob FROM integrations WHERE id=$1::uuid`, integ).Scan(&blob))
	dec, err := crypto.NewCredentialDecryptor(key)
	must(err)
	creds, err := dec.DecryptMap(blob)
	must(err)
	token := creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	accountID := creds["account_id"]
	if token == "" || accountID == "" {
		fatal("missing token/account_id")
	}

	base := "https://api.ringba.com/v2/" + accountID
	client := &http.Client{Timeout: 90 * time.Second}
	fmt.Printf("PHASE=G BASE=https://api.ringba.com/v2/{accountId} TOKEN=REDACTED SANDBOX=%s\n", callID)

	cols := []string{
		"inboundCallId", "campaignName", "campaignId", "publisherName", "targetName", "targetId",
		"conversionAmount", "payoutAmount", "hasConverted", "hasPayout",
		"wasConversionAdjusted", "wasPayoutAdjusted", "hasRecording", "hasAnnotations",
	}
	rec := mustFind(client, base, token, callID, cols)
	camp, _ := rec["campaignName"].(string)
	tgtID, _ := rec["targetId"].(string)
	fmt.Println("G0_RECORD_JSON_START")
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(redactRecord(rec))
	fmt.Println("G0_RECORD_JSON_END")

	assert("campaign sandbox", strings.EqualFold(camp, sandboxCamp), camp)
	assert("targetId sandbox", tgtID == sandboxTargetID, tgtID)
	assert("conversionAmount==0", asFloat(rec["conversionAmount"]) == 0, fmt.Sprint(rec["conversionAmount"]))
	assert("payoutAmount==0", asFloat(rec["payoutAmount"]) == 0, fmt.Sprint(rec["payoutAmount"]))
	assert("hasConverted absent", !hasKey(rec, "hasConverted"), fmt.Sprint(rec["hasConverted"]))
	assert("wasConversionAdjusted absent", !hasKey(rec, "wasConversionAdjusted"), fmt.Sprint(rec["wasConversionAdjusted"]))
	if !strings.EqualFold(camp, sandboxCamp) {
		fatal("ABORT: not sandbox campaign — refusing writes")
	}
	if asFloat(rec["conversionAmount"]) != 0 || asFloat(rec["payoutAmount"]) != 0 || hasKey(rec, "wasConversionAdjusted") {
		fatal("ABORT: call not clean — refusing convert fire")
	}

	st, _, whRaw, _ := do(client, http.MethodGet, base+"/WebHooks", token, nil)
	fmt.Printf("G1_WEBHOOKS status=%d ids=%v body=%s\n", st, webhookIDs(whRaw), snip(redactSecrets(whRaw), 1500))
	idsBefore := webhookIDs(whRaw)
	myHook := webhookByName(whRaw, "My Webhook")

	converted := false
	if myHook != "" {
		fmt.Printf("G2_FIRE_MY_WEBHOOK id=%s\n", myHook)
		fireHook(http.MethodGet, myHook, callID, "1.00")
		time.Sleep(4 * time.Second)
		after := mustFind(client, base, token, callID, cols)
		printRead("G2_READBACK_GET", after)
		converted = isConverted(after)
		fmt.Printf("G2_CONVERTED_AFTER_GET=%v\n", converted)
		if !converted {
			fireHook(http.MethodPost, myHook, callID, "1.00")
			time.Sleep(4 * time.Second)
			after = mustFind(client, base, token, callID, cols)
			printRead("G2_READBACK_POST", after)
			converted = isConverted(after)
			fmt.Printf("G2_CONVERTED_AFTER_POST=%v\n", converted)
		}
	} else {
		fmt.Println("G2_SKIP My Webhook not found")
	}

	createdID := ""
	if !converted {
		fmt.Println("G3_CREATE CALYX-SANDBOX-PHASE-G (clean query params)")
		body := map[string]interface{}{
			"name":                                 "CALYX-SANDBOX-PHASE-G",
			"incomingCallIdQueryStringParameter":   "call_id",
			"conversionAmountQueryStringParameter": "call_revenue",
			"conversionCriteriaParameterMatch":     "converted=yes",
		}
		st, _, raw, _ := do(client, http.MethodPost, base+"/WebHooks/CreateConversion", token, body)
		fmt.Printf("G3_CREATE status=%d body=%s\n", st, snip(redactSecrets(raw), 800))
		time.Sleep(500 * time.Millisecond)
		st, _, afterWH, _ := do(client, http.MethodGet, base+"/WebHooks", token, nil)
		fmt.Printf("G3_WEBHOOKS_AFTER status=%d ids=%v\n", st, webhookIDs(afterWH))
		created := diffIDs(idsBefore, webhookIDs(afterWH))
		fmt.Printf("G3_CREATED_IDS=%v PREEXISTING_UNTOUCHED=%v\n", created, preexistingUntouched(idsBefore, webhookIDs(afterWH)))
		if len(created) == 1 {
			createdID = created[0]
		} else {
			createdID = webhookByName(afterWH, "CALYX-SANDBOX-PHASE-G")
		}
		if createdID == "" {
			fatal("ABORT: could not identify created webhook")
		}
		st, _, urlRaw, _ := do(client, http.MethodPost, base+"/WebHooks/"+createdID+"/getUrl", token, map[string]interface{}{})
		fmt.Printf("G3_GETURL status=%d body=%s\n", st, snip(redactSecrets(urlRaw), 600))
		fireHook(http.MethodGet, createdID, callID, "1.00")
		time.Sleep(4 * time.Second)
		after := mustFind(client, base, token, callID, cols)
		printRead("G3_READBACK_GET", after)
		converted = isConverted(after)
		fmt.Printf("G3_CONVERTED_AFTER_GET=%v\n", converted)
		if !converted {
			fireHook(http.MethodPost, createdID, callID, "1.00")
			time.Sleep(4 * time.Second)
			after = mustFind(client, base, token, callID, cols)
			printRead("G3_READBACK_POST", after)
			converted = isConverted(after)
			fmt.Printf("G3_CONVERTED_AFTER_POST=%v\n", converted)
		}
	}

	final := mustFind(client, base, token, callID, cols)
	printRead("G4_FINAL", final)
	ok := isConverted(final) && approx(asFloat(final["conversionAmount"]), 1)
	fmt.Printf("PHASE_G_CONVERT_LOOP=%v hasConverted_present=%v val=%v conv=%v\n",
		ok, hasKey(final, "hasConverted"), valOr(final, "hasConverted"), valOr(final, "conversionAmount"))

	if createdID != "" {
		stDel, _, rawDel, _ := do(client, http.MethodDelete, base+"/WebHooks/"+createdID, token, nil)
		fmt.Printf("G5_DELETE_CREATED status=%d body=%s\n", stDel, snip(redactSecrets(rawDel), 300))
		time.Sleep(400 * time.Millisecond)
		st, _, afterDel, _ := do(client, http.MethodGet, base+"/WebHooks", token, nil)
		fmt.Printf("G5_WEBHOOKS_AFTER_DELETE ids=%v\n", webhookIDs(afterDel))
		_ = st
	} else {
		fmt.Println("G5_NO_CREATED_WEBHOOK skip delete")
	}
	fmt.Println("PHASE_G_COMPLETE")
}

func fireHook(method, hookID, callID, amount string) {
	q := url.Values{}
	q.Set("call_id", callID)
	q.Set("call_revenue", amount)
	q.Set("converted", "yes")
	fire := "https://webhook.ringba.com/hook/" + hookID + "?" + q.Encode()
	fmt.Printf("FIRE_URL=https://webhook.ringba.com/hook/%s?call_id=REDACTED&call_revenue=%s&converted=yes\n", hookID, amount)
	req, err := http.NewRequest(method, fire, nil)
	must(err)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("FIRE_%s ERR %v\n", method, err)
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	fmt.Printf("FIRE_%s status=%d body=%s\n", method, resp.StatusCode, snip(raw, 400))
}

func isConverted(rec map[string]interface{}) bool {
	if rec == nil {
		return false
	}
	if asFloat(rec["conversionAmount"]) >= 0.99 {
		return true
	}
	if v, ok := rec["hasConverted"]; ok {
		switch t := v.(type) {
		case bool:
			return t
		case string:
			return strings.EqualFold(t, "true")
		}
	}
	return false
}

func printRead(label string, rec map[string]interface{}) {
	fmt.Printf("%s conv=%v pay=%v hasConverted_present=%v val=%v wasAdj_present=%v campaign=%v\n",
		label, valOr(rec, "conversionAmount"), valOr(rec, "payoutAmount"),
		hasKey(rec, "hasConverted"), valOr(rec, "hasConverted"),
		hasKey(rec, "wasConversionAdjusted"), valOr(rec, "campaignName"))
}

func mustFind(client *http.Client, base, token, callID string, cols []string) map[string]interface{} {
	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-7 * 24 * time.Hour)
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}
	offset := 0
	for page := 0; page < 8; page++ {
		body := map[string]interface{}{
			"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
			"filters": []interface{}{}, "size": 1000, "offset": offset, "valueColumns": vc,
		}
		st, _, raw, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
		if st != 200 {
			fatal(fmt.Sprintf("calllogs HTTP %d %s", st, snip(raw, 200)))
		}
		var p struct {
			Report struct {
				Records []map[string]interface{} `json:"records"`
			} `json:"report"`
		}
		_ = json.Unmarshal(raw, &p)
		fmt.Printf("FIND page=%d records=%d offset=%d\n", page, len(p.Report.Records), offset)
		for _, row := range p.Report.Records {
			if id, _ := row["inboundCallId"].(string); id == callID {
				return row
			}
		}
		if len(p.Report.Records) < 1000 {
			break
		}
		offset += 1000
		time.Sleep(200 * time.Millisecond)
	}
	fatal("sandbox call ID not found in 7d call log")
	return nil
}

func do(client *http.Client, method, u, token string, body interface{}) (status int, location string, raw []byte, ctype string) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, rdr)
	must(err)
	req.Header.Set("Authorization", "Token "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", []byte(err.Error()), ""
	}
	defer resp.Body.Close()
	raw, _ = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return resp.StatusCode, resp.Header.Get("Location"), raw, resp.Header.Get("Content-Type")
}

func webhookIDs(raw []byte) []string {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	arr, _ := m["webHooks"].([]interface{})
	out := []string{}
	for _, it := range arr {
		obj, _ := it.(map[string]interface{})
		if id, ok := obj["id"].(string); ok {
			out = append(out, id)
		}
	}
	return out
}

func webhookByName(raw []byte, name string) string {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	arr, _ := m["webHooks"].([]interface{})
	for _, it := range arr {
		obj, _ := it.(map[string]interface{})
		n, _ := obj["name"].(string)
		id, _ := obj["id"].(string)
		if strings.EqualFold(n, name) {
			return id
		}
	}
	return ""
}

func diffIDs(before, after []string) []string {
	out := []string{}
	for _, a := range after {
		found := false
		for _, b := range before {
			if a == b {
				found = true
				break
			}
		}
		if !found {
			out = append(out, a)
		}
	}
	return out
}

func preexistingUntouched(before, after []string) bool {
	for _, id := range before {
		found := false
		for _, a := range after {
			if a == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func redactRecord(rec map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range rec {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "phone") || k == "callerId" {
			out[k] = "REDACTED_PII"
			continue
		}
		out[k] = v
	}
	return out
}

func redactSecrets(b []byte) []byte {
	return []byte(strings.ReplaceAll(string(b), "Token ", "Token REDACTED"))
}

func assert(name string, ok bool, got string) {
	if ok {
		fmt.Printf("ASSERT PASS %s got=%q\n", name, got)
	} else {
		fmt.Printf("ASSERT FAIL %s got=%q\n", name, got)
	}
}

func asFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		_, _ = fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return 0
	}
}

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.001
}

func hasKey(m map[string]interface{}, k string) bool {
	if m == nil {
		return false
	}
	_, ok := m[k]
	return ok
}

func valOr(m map[string]interface{}, k string) interface{} {
	if m == nil {
		return nil
	}
	return m[k]
}

func snip(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func must(err error) {
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "FATAL:", msg)
	os.Exit(1)
}
