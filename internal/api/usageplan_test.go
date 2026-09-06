package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/ratelimit"
)

// App usage-plan routes are admin-only (requireAdmin); instance
// routes also accept the target app's own token.

func TestUsagePlanRejectsNegativeLimits(t *testing.T) {
	srv, _, apps, _ := newAdminServer(t)
	c := srv.Client()

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"name":"bad","push_per_min":-1,"push_daily_quota":100}`))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative app plan: err=%v status=%d, want 400", err, resp.StatusCode)
	}

	created, err := apps.Create(context.Background(), "acme")
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	resp2, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+created.ID+"/usage-plans", "Bearer admintok",
		`{"name":"bad","push_per_min":10,"push_burst":-5,"push_daily_quota":0}`))
	if err != nil || resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative instance plan: err=%v status=%d, want 400", err, resp2.StatusCode)
	}

	// 0 is still accepted (means unlimited).
	resp3, err := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"name":"unlimited","push_per_min":0,"push_daily_quota":0}`))
	if err != nil || resp3.StatusCode != http.StatusCreated {
		t.Fatalf("zero limits should be accepted: err=%v status=%d", err, resp3.StatusCode)
	}
}

func TestAppUsagePlanCRUD(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"name":"gold","push_per_min":100,"push_daily_quota":5000}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("create plan: err=%v status=%d", err, resp.StatusCode)
	}
	var plan struct {
		ID                string  `json:"id"`
		Name              *string `json:"name"`
		AppPushPerMin     *int    `json:"push_per_min"`
		AppPushDailyQuota *int    `json:"push_daily_quota"`
	}
	dataOf(t, resp, &plan)
	if plan.ID == "" {
		t.Fatal("plan id should not be empty")
	}
	if plan.Name == nil || *plan.Name != "gold" {
		t.Fatalf("want name=gold, got %v", plan.Name)
	}
	if plan.AppPushPerMin == nil || *plan.AppPushPerMin != 100 {
		t.Fatalf("want per_min=100, got %v", plan.AppPushPerMin)
	}
	if plan.AppPushDailyQuota == nil || *plan.AppPushDailyQuota != 5000 {
		t.Fatalf("want daily_quota=5000, got %v", plan.AppPushDailyQuota)
	}

	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/usage-plans", "Bearer admintok", ""))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("list plans: err=%v status=%d", err, resp2.StatusCode)
	}
	var plans []struct {
		ID string `json:"id"`
	}
	dataOf(t, resp2, &plans)
	found := false
	for _, p := range plans {
		if p.ID == plan.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("created plan %q not found in list", plan.ID)
	}

	resp3, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/usage-plans/"+plan.ID, "Bearer admintok",
		`{"push_per_min":200,"push_daily_quota":null}`))
	if err != nil || resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("update plan: err=%v status=%d", err, resp3.StatusCode)
	}

	resp4, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/usage-plans/doesnotexist", "Bearer admintok",
		`{"push_per_min":1}`))
	if err != nil || resp4.StatusCode != http.StatusNotFound {
		t.Fatalf("update missing plan: err=%v status=%d", err, resp4.StatusCode)
	}

	resp5, err := c.Do(jsonReq(t, "DELETE", srv.URL+"/usage-plans/"+plan.ID, "Bearer admintok", ""))
	if err != nil || resp5.StatusCode != http.StatusNoContent {
		t.Fatalf("delete plan: err=%v status=%d", err, resp5.StatusCode)
	}

	resp6, err := c.Do(jsonReq(t, "DELETE", srv.URL+"/usage-plans/"+plan.ID, "Bearer admintok", ""))
	if err != nil || resp6.StatusCode != http.StatusNotFound {
		t.Fatalf("delete missing plan: err=%v status=%d", err, resp6.StatusCode)
	}
}

func TestDefaultAppUsagePlan(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	// The server test harness does not run startup seeding, so seed the global
	// default app plan directly (as main's startup would).
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.Queries.CreateDefaultAppUsagePlan(t.Context(), db.CreateDefaultAppUsagePlanParams{
		ID: "gdef", Name: "default",
		PushPerMin: 6000, CreatedAt: 1,
	}))

	resp, _ := c.Do(jsonReq(t, "GET", srv.URL+"/usage-plans", "Bearer admintok", ""))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: status=%d", resp.StatusCode)
	}
	var plans []struct {
		ID        string `json:"id"`
		IsDefault bool   `json:"is_default"`
	}
	dataOf(t, resp, &plans)
	found := false
	for _, p := range plans {
		if p.ID == "gdef" {
			found = p.IsDefault
		}
	}
	if !found {
		t.Fatalf("default app plan not listed with is_default=true: %+v", plans)
	}

	respDel, _ := c.Do(jsonReq(t, "DELETE", srv.URL+"/usage-plans/gdef", "Bearer admintok", ""))
	if respDel.StatusCode != http.StatusForbidden {
		t.Fatalf("delete default app plan: want 403, got %d", respDel.StatusCode)
	}
}

func TestAppUsagePlanCreateRequiresName(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	resp, err := srv.Client().Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok", `{}`))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got err=%v status=%d", err, resp.StatusCode)
	}
}

func TestAppUsagePlanCreateRequiresLimits(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()
	for _, body := range []string{
		`{"name":"n"}`,
		`{"name":"n","push_per_min":10}`,
		`{"name":"n","push_daily_quota":100}`,
	} {
		resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok", body))
		if err != nil || resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("body %s: want 400, got err=%v status=%d", body, err, resp.StatusCode)
		}
	}
}

func TestAssignAppUsagePlan(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	app1ID, _, _ := createAppAdmin(t, srv, `{"name":"app1"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"name":"shared","push_per_min":50,"push_daily_quota":0}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create shared plan: status=%d", resp.StatusCode)
	}
	var sharedPlan struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &sharedPlan)

	resp2, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+app1ID, "Bearer admintok",
		`{"usage_plan_id":"nosuchplan"}`))
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("assign missing plan: want 404, got %d", resp2.StatusCode)
	}

	resp3, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+app1ID, "Bearer admintok",
		`{"usage_plan_id":"`+sharedPlan.ID+`"}`))
	if resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("assign shared plan to app1: want 204, got %d", resp3.StatusCode)
	}

	resp4, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/bogusapp", "Bearer admintok",
		`{"usage_plan_id":"`+sharedPlan.ID+`"}`))
	if resp4.StatusCode != http.StatusNotFound {
		t.Fatalf("assign to missing app: want 404, got %d", resp4.StatusCode)
	}

	resp5, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+app1ID, "Bearer admintok", `{"usage_plan_id":""}`))
	if resp5.StatusCode != http.StatusNoContent {
		t.Fatalf("unassign plan: want 204, got %d", resp5.StatusCode)
	}

	app2ID, _, pat := createAppAdmin(t, srv, `{"name":"app2"}`)
	resp6, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+app2ID, "Bearer "+pat,
		`{"usage_plan_id":"`+sharedPlan.ID+`"}`))
	if resp6.StatusCode != http.StatusForbidden {
		t.Fatalf("pat assign plan: want 403, got %d", resp6.StatusCode)
	}

	resp7, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+app2ID, "Bearer "+pat, `{"name":"renamed"}`))
	if resp7.StatusCode != http.StatusNoContent {
		t.Fatalf("pat rename app: want 204, got %d", resp7.StatusCode)
	}
}

func TestAssignBespokePlanToOtherApp(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	app1ID, _, _ := createAppAdmin(t, srv, `{"name":"appA"}`)
	app2ID, _, _ := createAppAdmin(t, srv, `{"name":"appB"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"app_id":"`+app2ID+`","name":"appB bespoke","push_per_min":5,"push_daily_quota":0}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create app2 scoped plan: status=%d", resp.StatusCode)
	}
	var bespoke struct {
		ID    string  `json:"id"`
		AppID *string `json:"app_id"`
	}
	dataOf(t, resp, &bespoke)
	if bespoke.AppID == nil || *bespoke.AppID != app2ID {
		t.Fatalf("scoped plan app_id: want %q, got %v", app2ID, bespoke.AppID)
	}

	respDup, _ := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"app_id":"`+app2ID+`","name":"appB bespoke 2","push_per_min":6,"push_daily_quota":0}`))
	if respDup.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate scoped plan: want 409, got %d", respDup.StatusCode)
	}

	respMiss, _ := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"app_id":"nosuchapp","name":"ghost","push_per_min":1,"push_daily_quota":0}`))
	if respMiss.StatusCode != http.StatusNotFound {
		t.Fatalf("scoped plan for unknown app: want 404, got %d", respMiss.StatusCode)
	}

	resp2, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+app1ID, "Bearer admintok",
		`{"usage_plan_id":"`+bespoke.ID+`"}`))
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("assign other app bespoke plan: want 403, got %d", resp2.StatusCode)
	}
}

func TestAppUsagePlanAdminOnly(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	_, _, pat := createAppAdmin(t, srv, `{"name":"patapp"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"name":"planX","push_per_min":0,"push_daily_quota":0}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin create plan: status=%d", resp.StatusCode)
	}
	var plan struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &plan)

	appID, _, _ := createAppAdmin(t, srv, `{"name":"assignapp"}`)

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/usage-plans", `{"name":"x"}`},
		{"GET", "/usage-plans", ""},
		{"PATCH", "/usage-plans/" + plan.ID, `{"name":"y"}`},
		{"DELETE", "/usage-plans/" + plan.ID, ""},
		{"PATCH", "/apps/" + appID, `{"usage_plan_id":"` + plan.ID + `"}`},
	}

	for _, ep := range endpoints {
		r := jsonReq(t, ep.method, srv.URL+ep.path, "Bearer "+pat, ep.body)
		res, err := c.Do(r)
		if err != nil {
			t.Fatalf("%s %s: err=%v", ep.method, ep.path, err)
		}
		if res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s with pat token: want 401/403, got %d", ep.method, ep.path, res.StatusCode)
		}
	}
}

func TestInstanceUsagePlanCRUD(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, pat := createAppAdmin(t, srv, `{"name":"iplanapp"}`)

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok",
		`{"name":"silver","push_per_min":30,"push_burst":5,"push_daily_quota":0}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin create instance plan: err=%v status=%d", err, resp.StatusCode)
	}
	var plan struct {
		ID                 string  `json:"id"`
		Name               *string `json:"name"`
		InstancePushPerMin *int    `json:"push_per_min"`
		InstancePushBurst  *int    `json:"push_burst"`
	}
	dataOf(t, resp, &plan)
	if plan.ID == "" {
		t.Fatal("plan id should not be empty")
	}
	if plan.Name == nil || *plan.Name != "silver" {
		t.Fatalf("want name=silver, got %v", plan.Name)
	}
	if plan.InstancePushPerMin == nil || *plan.InstancePushPerMin != 30 {
		t.Fatalf("want per_min=30, got %v", plan.InstancePushPerMin)
	}
	if plan.InstancePushBurst == nil || *plan.InstancePushBurst != 5 {
		t.Fatalf("want burst=5, got %v", plan.InstancePushBurst)
	}

	resp2, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer "+pat,
		`{"name":"bronze","push_per_min":10,"push_burst":0,"push_daily_quota":0}`))
	if err != nil || resp2.StatusCode != http.StatusCreated {
		t.Fatalf("pat create instance plan: err=%v status=%d", err, resp2.StatusCode)
	}
	var plan2 struct {
		ID string `json:"id"`
	}
	dataOf(t, resp2, &plan2)

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok", ""))
	if err != nil || resp3.StatusCode != http.StatusOK {
		t.Fatalf("list instance plans: err=%v status=%d", err, resp3.StatusCode)
	}
	var plans []struct {
		ID string `json:"id"`
	}
	dataOf(t, resp3, &plans)
	findID := func(id string) bool {
		for _, p := range plans {
			if p.ID == id {
				return true
			}
		}
		return false
	}
	if !findID(plan.ID) || !findID(plan2.ID) {
		t.Fatalf("plans not found in list: got %v", plans)
	}

	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer "+pat, ""))
	if err != nil || resp4.StatusCode != http.StatusOK {
		t.Fatalf("pat list instance plans: err=%v status=%d", err, resp4.StatusCode)
	}

	resp5, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/usage-plans/"+plan.ID, "Bearer admintok",
		`{"push_per_min":null,"push_burst":10}`))
	if err != nil || resp5.StatusCode != http.StatusNoContent {
		t.Fatalf("admin update instance plan: err=%v status=%d", err, resp5.StatusCode)
	}

	resp6, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/usage-plans/"+plan2.ID, "Bearer "+pat,
		`{"push_burst":3}`))
	if err != nil || resp6.StatusCode != http.StatusNoContent {
		t.Fatalf("pat update instance plan: err=%v status=%d", err, resp6.StatusCode)
	}

	resp7, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/usage-plans/nosuchplan", "Bearer admintok",
		`{"push_burst":1}`))
	if err != nil || resp7.StatusCode != http.StatusNotFound {
		t.Fatalf("update missing instance plan: err=%v status=%d", err, resp7.StatusCode)
	}

	resp8, err := c.Do(jsonReq(t, "DELETE", srv.URL+"/apps/"+appID+"/usage-plans/"+plan2.ID, "Bearer "+pat, ""))
	if err != nil || resp8.StatusCode != http.StatusNoContent {
		t.Fatalf("pat delete instance plan: err=%v status=%d", err, resp8.StatusCode)
	}

	resp9, err := c.Do(jsonReq(t, "DELETE", srv.URL+"/apps/"+appID+"/usage-plans/"+plan2.ID, "Bearer admintok", ""))
	if err != nil || resp9.StatusCode != http.StatusNotFound {
		t.Fatalf("delete missing instance plan: err=%v status=%d", err, resp9.StatusCode)
	}

	resp10, err := c.Do(jsonReq(t, "DELETE", srv.URL+"/apps/"+appID+"/usage-plans/"+plan.ID, "Bearer admintok", ""))
	if err != nil || resp10.StatusCode != http.StatusNoContent {
		t.Fatalf("admin delete instance plan: err=%v status=%d", err, resp10.StatusCode)
	}
}

func TestInstanceUsagePlanCreateRequiresName(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	appID, _, _ := createAppAdmin(t, srv, `{"name":"namecheck"}`)
	resp, err := srv.Client().Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok", `{}`))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got err=%v status=%d", err, resp.StatusCode)
	}
}

func TestInstanceUsagePlanCreateRequiresLimits(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()
	appID, _, _ := createAppAdmin(t, srv, `{"name":"limitcheck"}`)
	for _, body := range []string{
		`{"name":"n"}`,
		`{"name":"n","push_per_min":10,"push_burst":2}`,
		`{"name":"n","push_per_min":10,"push_daily_quota":1}`,
		`{"name":"n","push_burst":2,"push_daily_quota":1}`,
	} {
		resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok", body))
		if err != nil || resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("body %s: want 400, got err=%v status=%d", body, err, resp.StatusCode)
		}
	}
}

func TestInstanceUsagePlanCrossApp(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appAID, _, patA := createAppAdmin(t, srv, `{"name":"appA"}`)
	appBID, _, patB := createAppAdmin(t, srv, `{"name":"appB"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/usage-plans", "Bearer admintok",
		`{"name":"planA","push_per_min":20,"push_burst":0,"push_daily_quota":0}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create appA plan: status=%d", resp.StatusCode)
	}
	var planA struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &planA)

	crossEndpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/apps/" + appAID + "/usage-plans", `{"name":"x"}`},
		{"GET", "/apps/" + appAID + "/usage-plans", ""},
		{"PATCH", "/apps/" + appAID + "/usage-plans/" + planA.ID, `{"push_burst":1}`},
		{"DELETE", "/apps/" + appAID + "/usage-plans/" + planA.ID, ""},
	}
	for _, ep := range crossEndpoints {
		res, err := c.Do(jsonReq(t, ep.method, srv.URL+ep.path, "Bearer "+patB, ep.body))
		if err != nil {
			t.Fatalf("%s %s: err=%v", ep.method, ep.path, err)
		}
		if res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s with appB pat: want 401/403, got %d", ep.method, ep.path, res.StatusCode)
		}
	}

	resp2, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appBID+"/usage-plans/"+planA.ID, "Bearer admintok",
		`{"push_burst":1}`))
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("update planA via appB path: want 404, got %d", resp2.StatusCode)
	}

	resp3, _ := c.Do(jsonReq(t, "DELETE", srv.URL+"/apps/"+appBID+"/usage-plans/"+planA.ID, "Bearer admintok", ""))
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("delete planA via appB path: want 404, got %d", resp3.StatusCode)
	}

	resp4, _ := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appBID+"/usage-plans", "Bearer "+patA, ""))
	if resp4.StatusCode != http.StatusUnauthorized && resp4.StatusCode != http.StatusForbidden {
		t.Errorf("appA pat list appB plans: want 401/403, got %d", resp4.StatusCode)
	}
}

func TestInstanceScopedPlanLifecycle(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, _ := createAppAdmin(t, srv, `{"name":"ownchk"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/instances", "Bearer admintok", `{"label":"svc"}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: status=%d", resp.StatusCode)
	}
	var inst struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &inst)

	resp2, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok",
		`{"instance_id":"`+inst.ID+`","name":"svc plan","push_per_min":5,"push_burst":1,"push_daily_quota":0}`))
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("create instance-scoped plan: status=%d", resp2.StatusCode)
	}
	var scoped struct {
		ID         string  `json:"id"`
		InstanceID *string `json:"instance_id"`
	}
	dataOf(t, resp2, &scoped)
	if scoped.InstanceID == nil || *scoped.InstanceID != inst.ID {
		t.Fatalf("scoped plan instance_id: want %q, got %v", inst.ID, scoped.InstanceID)
	}

	respDup, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok",
		`{"instance_id":"`+inst.ID+`","name":"svc plan 2","push_per_min":6,"push_burst":1,"push_daily_quota":0}`))
	if respDup.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate instance-scoped plan: want 409, got %d", respDup.StatusCode)
	}

	respMiss, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok",
		`{"instance_id":"nosuchinst","name":"ghost","push_per_min":1,"push_burst":1,"push_daily_quota":0}`))
	if respMiss.StatusCode != http.StatusNotFound {
		t.Fatalf("scoped plan for unknown instance: want 404, got %d", respMiss.StatusCode)
	}

	resp3, _ := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok", ""))
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("list plans: status=%d", resp3.StatusCode)
	}
	var plans []struct {
		ID         string  `json:"id"`
		InstanceID *string `json:"instance_id"`
	}
	dataOf(t, resp3, &plans)
	found := false
	for _, p := range plans {
		if p.ID == scoped.ID {
			found = p.InstanceID != nil && *p.InstanceID == inst.ID
		}
	}
	if !found {
		t.Fatalf("scoped plan %q not listed with its instance_id", scoped.ID)
	}

	resp4, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/usage-plans/"+scoped.ID, "Bearer admintok",
		`{"push_burst":2}`))
	if resp4.StatusCode != http.StatusNoContent {
		t.Errorf("update instance-scoped plan via app route: want 204, got %d", resp4.StatusCode)
	}

	resp5, _ := c.Do(jsonReq(t, "DELETE", srv.URL+"/apps/"+appID+"/usage-plans/"+scoped.ID, "Bearer admintok", ""))
	if resp5.StatusCode != http.StatusNoContent {
		t.Errorf("delete instance-scoped plan via app route: want 204, got %d", resp5.StatusCode)
	}
}

func TestAssignInstanceUsagePlanEndpoint(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appAID, _, patA := createAppAdmin(t, srv, `{"name":"assignA"}`)
	appBID, _, _ := createAppAdmin(t, srv, `{"name":"assignB"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/instances", "Bearer admintok", `{"label":"svc"}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: status=%d", resp.StatusCode)
	}
	var inst struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &inst)

	resp2, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/usage-plans", "Bearer admintok",
		`{"name":"planA","push_per_min":15,"push_burst":3,"push_daily_quota":0}`))
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("create shared instance plan: status=%d", resp2.StatusCode)
	}
	var sharedPlan struct {
		ID string `json:"id"`
	}
	dataOf(t, resp2, &sharedPlan)

	resp3, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appBID+"/usage-plans", "Bearer admintok",
		`{"name":"planB","push_per_min":99,"push_burst":0,"push_daily_quota":0}`))
	if resp3.StatusCode != http.StatusCreated {
		t.Fatalf("create appB shared instance plan: status=%d", resp3.StatusCode)
	}
	var planB struct {
		ID string `json:"id"`
	}
	dataOf(t, resp3, &planB)

	resp4, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appAID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"nosuchplan"}`))
	if resp4.StatusCode != http.StatusNotFound {
		t.Errorf("assign missing plan: want 404, got %d", resp4.StatusCode)
	}

	// plan lookup is scoped to the path app, so appB's plan is not found under appA → 404.
	resp5, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appAID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"`+planB.ID+`"}`))
	if resp5.StatusCode != http.StatusNotFound {
		t.Errorf("assign other-app plan: want 404, got %d", resp5.StatusCode)
	}

	resp6, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/usage-plans", "Bearer admintok",
		`{"instance_id":"`+inst.ID+`","name":"inst plan","push_per_min":7,"push_burst":1,"push_daily_quota":0}`))
	if resp6.StatusCode != http.StatusCreated {
		t.Fatalf("create instance-scoped plan: status=%d", resp6.StatusCode)
	}
	var scopedOwn struct {
		ID string `json:"id"`
	}
	dataOf(t, resp6, &scopedOwn)
	resp7, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appAID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"`+scopedOwn.ID+`"}`))
	if resp7.StatusCode != http.StatusNoContent {
		t.Errorf("assign own instance-scoped plan: want 204, got %d", resp7.StatusCode)
	}

	resp8, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/instances", "Bearer admintok", `{"label":"svc2"}`))
	if resp8.StatusCode != http.StatusCreated {
		t.Fatalf("create second instance: status=%d", resp8.StatusCode)
	}
	var inst2 struct {
		ID string `json:"id"`
	}
	dataOf(t, resp8, &inst2)
	resp9, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appAID+"/usage-plans", "Bearer admintok",
		`{"instance_id":"`+inst2.ID+`","name":"inst2 plan","push_per_min":8,"push_burst":1,"push_daily_quota":0}`))
	if resp9.StatusCode != http.StatusCreated {
		t.Fatalf("create inst2-scoped plan: status=%d", resp9.StatusCode)
	}
	var scopedOther struct {
		ID string `json:"id"`
	}
	dataOf(t, resp9, &scopedOther)
	resp10, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appAID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"`+scopedOther.ID+`"}`))
	if resp10.StatusCode != http.StatusNotFound {
		t.Errorf("assign other-instance scoped plan: want 404, got %d", resp10.StatusCode)
	}

	resp11, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appAID+"/instances/"+inst.ID, "Bearer "+patA,
		`{"usage_plan_id":"`+sharedPlan.ID+`"}`))
	if resp11.StatusCode != http.StatusNoContent {
		t.Errorf("pat assign shared plan: want 204, got %d", resp11.StatusCode)
	}

	resp12, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appAID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"`+sharedPlan.ID+`"}`))
	if resp12.StatusCode != http.StatusNoContent {
		t.Errorf("admin assign shared plan: want 204, got %d", resp12.StatusCode)
	}

	resp13, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appBID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"`+planB.ID+`"}`))
	if resp13.StatusCode != http.StatusForbidden {
		t.Errorf("assign cross-app instance: want 403, got %d", resp13.StatusCode)
	}

	resp14, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appAID+"/instances/"+inst.ID, "Bearer admintok", `{"usage_plan_id":""}`))
	if resp14.StatusCode != http.StatusNoContent {
		t.Errorf("unassign plan: want 204, got %d", resp14.StatusCode)
	}
}

func TestDefaultInstanceUsagePlan(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, _ := createAppAdmin(t, srv, `{"name":"defapp"}`)

	resp, _ := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok", ""))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list plans: status=%d", resp.StatusCode)
	}
	var plans []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
	}
	dataOf(t, resp, &plans)
	var defID string
	for _, p := range plans {
		if p.IsDefault {
			defID = p.ID
		}
	}
	if defID == "" {
		t.Fatalf("expected a seeded default plan, got %+v", plans)
	}

	respDel, _ := c.Do(jsonReq(t, "DELETE", srv.URL+"/apps/"+appID+"/usage-plans/"+defID, "Bearer admintok", ""))
	if respDel.StatusCode != http.StatusForbidden {
		t.Fatalf("delete default plan: want 403, got %d", respDel.StatusCode)
	}

	respUp, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/usage-plans/"+defID, "Bearer admintok",
		`{"push_per_min":25,"push_burst":4}`))
	if respUp.StatusCode != http.StatusNoContent {
		t.Fatalf("update default plan: status=%d", respUp.StatusCode)
	}

	ctx := context.Background()
	resolver := ratelimit.NewResolver(d.Queries)
	got := resolver.Limits(ctx, appID, "")
	if got.InstancePushPerMin != 25 || got.InstancePushBurst != 4 {
		t.Errorf("resolver: want per-min=25 burst=4 from default, got %+v", got)
	}
}

func TestInstanceUsagePlanEndToEnd(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, _ := createAppAdmin(t, srv, `{"name":"e2eapp"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/instances", "Bearer admintok", `{"label":"svc"}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: status=%d", resp.StatusCode)
	}
	var inst struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &inst)

	resp2, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok",
		`{"name":"e2e-plan","push_per_min":40,"push_burst":8,"push_daily_quota":0}`))
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("create plan: status=%d", resp2.StatusCode)
	}
	var plan struct {
		ID string `json:"id"`
	}
	dataOf(t, resp2, &plan)

	resp3, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"`+plan.ID+`"}`))
	if resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("assign plan to instance: want 204, got %d", resp3.StatusCode)
	}

	ctx := context.Background()
	instRow, err := d.Queries.GetInstance(ctx, inst.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if !instRow.UsagePlanID.Valid || instRow.UsagePlanID.String != plan.ID {
		t.Fatalf("instance usage_plan_id: want %q, got %v", plan.ID, instRow.UsagePlanID)
	}

	resolver := ratelimit.NewResolver(d.Queries)
	got := resolver.Limits(ctx, appID, instRow.UsagePlanID.String)
	if got.InstancePushPerMin != 40 {
		t.Errorf("resolver: want push_per_min=40, got %d", got.InstancePushPerMin)
	}
	if got.InstancePushBurst != 8 {
		t.Errorf("resolver: want push_burst=8, got %d", got.InstancePushBurst)
	}
}

func TestAppScopedUsagePlanEndToEnd(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, _ := createAppAdmin(t, srv, `{"name":"scoped-app"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"app_id":"`+appID+`","name":"app bespoke","push_per_min":10,"push_daily_quota":1000}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create app-scoped plan: status=%d", resp.StatusCode)
	}
	var plan struct {
		ID        string  `json:"id"`
		AppID     *string `json:"app_id"`
		IsDefault bool    `json:"is_default"`
	}
	dataOf(t, resp, &plan)
	if plan.AppID == nil || *plan.AppID != appID {
		t.Fatalf("scoped plan app_id: want %q, got %v", appID, plan.AppID)
	}
	if plan.IsDefault {
		t.Fatal("scoped plan must not be default")
	}

	resp2, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID, "Bearer admintok",
		`{"usage_plan_id":"`+plan.ID+`"}`))
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("assign scoped plan: want 204, got %d", resp2.StatusCode)
	}

	ctx := context.Background()
	app1, err := d.Queries.GetApp(ctx, appID)
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	if !app1.UsagePlanID.Valid || app1.UsagePlanID.String != plan.ID {
		t.Fatalf("expected usage_plan_id=%q, got %v", plan.ID, app1.UsagePlanID)
	}

	resolver := ratelimit.NewResolver(d.Queries)
	got := resolver.Limits(ctx, appID, "")
	if got.AppPushPerMin != 10 || got.AppPushDailyQuota != 1000 {
		t.Errorf("resolver: want per-min=10 daily=1000, got %+v", got)
	}

	resp3, _ := c.Do(jsonReq(t, "DELETE", srv.URL+"/usage-plans/"+plan.ID, "Bearer admintok", ""))
	if resp3.StatusCode != http.StatusConflict {
		t.Errorf("delete assigned scoped plan: want 409, got %d", resp3.StatusCode)
	}

	resp4, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID, "Bearer admintok", `{"usage_plan_id":""}`))
	if resp4.StatusCode != http.StatusNoContent {
		t.Fatalf("unassign: want 204, got %d", resp4.StatusCode)
	}
	resp5, _ := c.Do(jsonReq(t, "DELETE", srv.URL+"/usage-plans/"+plan.ID, "Bearer admintok", ""))
	if resp5.StatusCode != http.StatusNoContent {
		t.Errorf("delete unassigned scoped plan: want 204, got %d", resp5.StatusCode)
	}
}

func TestInstanceScopedUsagePlanEndToEnd(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, _ := createAppAdmin(t, srv, `{"name":"inst-scoped-e2e"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/instances", "Bearer admintok", `{"label":"svc"}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: status=%d", resp.StatusCode)
	}
	var inst struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &inst)

	resp2, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok",
		`{"instance_id":"`+inst.ID+`","name":"inst bespoke","push_per_min":12,"push_burst":3,"push_daily_quota":0}`))
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("create instance-scoped plan: status=%d", resp2.StatusCode)
	}
	var plan struct {
		ID string `json:"id"`
	}
	dataOf(t, resp2, &plan)

	resp3, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"`+plan.ID+`"}`))
	if resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("assign instance-scoped plan: want 204, got %d", resp3.StatusCode)
	}

	ctx := context.Background()
	instRow, err := d.Queries.GetInstance(ctx, inst.ID)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if !instRow.UsagePlanID.Valid || instRow.UsagePlanID.String != plan.ID {
		t.Fatalf("expected usage_plan_id=%q, got %v", plan.ID, instRow.UsagePlanID)
	}

	resolver := ratelimit.NewResolver(d.Queries)
	got := resolver.Limits(ctx, appID, plan.ID)
	if got.InstancePushPerMin != 12 || got.InstancePushBurst != 3 {
		t.Errorf("resolver: want per-min=12 burst=3, got %+v", got)
	}
}

func TestGetAssignedAppUsagePlan(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, pat := createAppAdmin(t, srv, `{"name":"readback"}`)

	// Creating the app ensures the global default app plan exists, so with no
	// assignment the readback falls back to it (0/unlimited limits).
	resp0, _ := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/usage-plan", "Bearer "+pat, ""))
	if resp0.StatusCode != http.StatusOK {
		t.Fatalf("readback with no assignment: want 200, got %d", resp0.StatusCode)
	}
	var def struct {
		IsDefault      bool `json:"is_default"`
		PushPerMin     int  `json:"push_per_min"`
		PushDailyQuota int  `json:"push_daily_quota"`
	}
	dataOf(t, resp0, &def)
	if !def.IsDefault || def.PushPerMin != 0 || def.PushDailyQuota != 0 {
		t.Fatalf("readback default fallback: got %+v", def)
	}

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"app_id":"`+appID+`","name":"readback plan","push_per_min":42,"push_daily_quota":900}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create scoped plan: status=%d", resp.StatusCode)
	}
	var plan struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &plan)
	resp2, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID, "Bearer admintok",
		`{"usage_plan_id":"`+plan.ID+`"}`))
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("assign plan: status=%d", resp2.StatusCode)
	}

	resp3, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/usage-plan", "Bearer "+pat, ""))
	if err != nil || resp3.StatusCode != http.StatusOK {
		t.Fatalf("readback via pat: err=%v status=%d", err, resp3.StatusCode)
	}
	var got struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		PushPerMin     int    `json:"push_per_min"`
		PushDailyQuota int    `json:"push_daily_quota"`
	}
	dataOf(t, resp3, &got)
	if got.ID != plan.ID || got.Name != "readback plan" ||
		got.PushPerMin != 42 || got.PushDailyQuota != 900 {
		t.Errorf("readback: got %+v", got)
	}

	resp4, _ := c.Do(jsonReq(t, "GET", srv.URL+"/apps/nosuchapp/usage-plan", "Bearer admintok", ""))
	if resp4.StatusCode != http.StatusNotFound {
		t.Errorf("readback unknown app: want 404, got %d", resp4.StatusCode)
	}
}

func TestGetAssignedAppUsagePlanDefaultFallback(t *testing.T) {
	srv, d, _, _ := newAdminServer(t)
	c := srv.Client()

	// Seed the global default app plan as main's startup would.
	if err := d.Queries.CreateDefaultAppUsagePlan(t.Context(), db.CreateDefaultAppUsagePlanParams{
		ID: "gdef", Name: "default", PushPerMin: 6000, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	appID, _, pat := createAppAdmin(t, srv, `{"name":"deffallback"}`)

	resp, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/usage-plan", "Bearer "+pat, ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("readback fallback: err=%v status=%d", err, resp.StatusCode)
	}
	var got struct {
		ID        string `json:"id"`
		IsDefault bool   `json:"is_default"`
	}
	dataOf(t, resp, &got)
	if got.ID != "gdef" || !got.IsDefault {
		t.Errorf("want default plan gdef, got %+v", got)
	}
}

func TestGetAssignedInstanceUsagePlan(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, pat := createAppAdmin(t, srv, `{"name":"inst-readback"}`)

	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/instances", "Bearer admintok", `{"label":"svc"}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: status=%d", resp.StatusCode)
	}
	var inst struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &inst)

	// Nothing assigned yet → the app's seeded default instance plan.
	resp0, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/instances/"+inst.ID+"/usage-plan", "Bearer "+pat, ""))
	if err != nil || resp0.StatusCode != http.StatusOK {
		t.Fatalf("readback fallback: err=%v status=%d", err, resp0.StatusCode)
	}
	var def struct {
		ID        string `json:"id"`
		IsDefault bool   `json:"is_default"`
	}
	dataOf(t, resp0, &def)
	if !def.IsDefault || def.ID == "" {
		t.Errorf("want seeded default plan, got %+v", def)
	}

	resp2, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/usage-plans", "Bearer admintok",
		`{"name":"ir plan","push_per_min":33,"push_burst":6,"push_daily_quota":0}`))
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("create shared instance plan: status=%d", resp2.StatusCode)
	}
	var plan struct {
		ID string `json:"id"`
	}
	dataOf(t, resp2, &plan)
	resp3, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/instances/"+inst.ID, "Bearer admintok",
		`{"usage_plan_id":"`+plan.ID+`"}`))
	if resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("assign plan: status=%d", resp3.StatusCode)
	}
	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/instances/"+inst.ID+"/usage-plan", "Bearer "+pat, ""))
	if err != nil || resp4.StatusCode != http.StatusOK {
		t.Fatalf("readback via pat: err=%v status=%d", err, resp4.StatusCode)
	}
	var got struct {
		ID         string `json:"id"`
		PushPerMin int    `json:"push_per_min"`
		PushBurst  int    `json:"push_burst"`
	}
	dataOf(t, resp4, &got)
	if got.ID != plan.ID || got.PushPerMin != 33 || got.PushBurst != 6 {
		t.Errorf("readback: got %+v", got)
	}

	resp5, _ := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/instances/nosuchinst/usage-plan", "Bearer admintok", ""))
	if resp5.StatusCode != http.StatusNotFound {
		t.Errorf("readback unknown instance: want 404, got %d", resp5.StatusCode)
	}
	appBID, _, patB := createAppAdmin(t, srv, `{"name":"otherapp"}`)
	resp6, _ := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appBID+"/instances/"+inst.ID+"/usage-plan", "Bearer "+patB, ""))
	if resp6.StatusCode != http.StatusForbidden {
		t.Errorf("readback cross-app instance: want 403, got %d", resp6.StatusCode)
	}
}

func TestUpdateInstanceLabel(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	appID, _, pat := createAppAdmin(t, srv, `{"name":"labelupd"}`)
	resp, _ := c.Do(jsonReq(t, "POST", srv.URL+"/apps/"+appID+"/instances", "Bearer admintok", `{"label":"svc"}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create instance: status=%d", resp.StatusCode)
	}
	var inst struct {
		ID string `json:"id"`
	}
	dataOf(t, resp, &inst)

	resp2, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/instances/"+inst.ID, "Bearer "+pat,
		`{"label":"prod"}`))
	if err != nil || resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("rename instance: err=%v status=%d", err, resp2.StatusCode)
	}
	resp3, _ := c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+appID+"/instances/"+inst.ID, "Bearer admintok", ""))
	var got struct {
		Label string `json:"label"`
	}
	dataOf(t, resp3, &got)
	if got.Label != "prod" {
		t.Errorf("label not updated: got %q", got.Label)
	}

	resp4, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/instances/"+inst.ID, "Bearer admintok", `{"label":""}`))
	if resp4.StatusCode != http.StatusBadRequest {
		t.Errorf("empty label: want 400, got %d", resp4.StatusCode)
	}
	resp5, _ := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+appID+"/instances/"+inst.ID, "Bearer admintok", `{}`))
	if resp5.StatusCode != http.StatusBadRequest {
		t.Errorf("empty body: want 400, got %d", resp5.StatusCode)
	}
}

func TestAppUsagePlanCRUDSelfRegSubscribe(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	resp, err := c.Do(jsonReq(t, "POST", srv.URL+"/usage-plans", "Bearer admintok",
		`{"name":"selfregplan","push_per_min":10,"push_daily_quota":0,"register_per_min":5,"register_burst":2,"register_ip_per_min":3,"register_ip_burst":1,"subscribe_per_min":8,"subscribe_burst":4}`))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("create plan: err=%v status=%d", err, resp.StatusCode)
	}
	var plan struct {
		ID               string `json:"id"`
		RegisterPerMin   *int   `json:"register_per_min"`
		RegisterBurst    *int   `json:"register_burst"`
		RegisterIPPerMin *int   `json:"register_ip_per_min"`
		RegisterIPBurst  *int   `json:"register_ip_burst"`
		SubscribePerMin  *int   `json:"subscribe_per_min"`
		SubscribeBurst   *int   `json:"subscribe_burst"`
	}
	dataOf(t, resp, &plan)
	if plan.ID == "" {
		t.Fatal("plan id should not be empty")
	}
	if plan.RegisterPerMin == nil || *plan.RegisterPerMin != 5 {
		t.Fatalf("create: want register_per_min=5, got %v", plan.RegisterPerMin)
	}
	if plan.RegisterBurst == nil || *plan.RegisterBurst != 2 {
		t.Fatalf("create: want register_burst=2, got %v", plan.RegisterBurst)
	}
	if plan.RegisterIPPerMin == nil || *plan.RegisterIPPerMin != 3 {
		t.Fatalf("create: want register_ip_per_min=3, got %v", plan.RegisterIPPerMin)
	}
	if plan.RegisterIPBurst == nil || *plan.RegisterIPBurst != 1 {
		t.Fatalf("create: want register_ip_burst=1, got %v", plan.RegisterIPBurst)
	}
	if plan.SubscribePerMin == nil || *plan.SubscribePerMin != 8 {
		t.Fatalf("create: want subscribe_per_min=8, got %v", plan.SubscribePerMin)
	}
	if plan.SubscribeBurst == nil || *plan.SubscribeBurst != 4 {
		t.Fatalf("create: want subscribe_burst=4, got %v", plan.SubscribeBurst)
	}

	resp2, err := c.Do(jsonReq(t, "GET", srv.URL+"/usage-plans/"+plan.ID, "Bearer admintok", ""))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("get plan: err=%v status=%d", err, resp2.StatusCode)
	}
	var got struct {
		RegisterPerMin   *int `json:"register_per_min"`
		RegisterBurst    *int `json:"register_burst"`
		RegisterIPPerMin *int `json:"register_ip_per_min"`
		RegisterIPBurst  *int `json:"register_ip_burst"`
		SubscribePerMin  *int `json:"subscribe_per_min"`
		SubscribeBurst   *int `json:"subscribe_burst"`
	}
	dataOf(t, resp2, &got)
	if got.RegisterPerMin == nil || *got.RegisterPerMin != 5 {
		t.Errorf("GET: want register_per_min=5, got %v", got.RegisterPerMin)
	}
	if got.RegisterBurst == nil || *got.RegisterBurst != 2 {
		t.Errorf("GET: want register_burst=2, got %v", got.RegisterBurst)
	}
	if got.RegisterIPPerMin == nil || *got.RegisterIPPerMin != 3 {
		t.Errorf("GET: want register_ip_per_min=3, got %v", got.RegisterIPPerMin)
	}
	if got.RegisterIPBurst == nil || *got.RegisterIPBurst != 1 {
		t.Errorf("GET: want register_ip_burst=1, got %v", got.RegisterIPBurst)
	}
	if got.SubscribePerMin == nil || *got.SubscribePerMin != 8 {
		t.Errorf("GET: want subscribe_per_min=8, got %v", got.SubscribePerMin)
	}
	if got.SubscribeBurst == nil || *got.SubscribeBurst != 4 {
		t.Errorf("GET: want subscribe_burst=4, got %v", got.SubscribeBurst)
	}

	resp3, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/usage-plans/"+plan.ID, "Bearer admintok",
		`{"register_per_min":99}`))
	if err != nil || resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("update plan: err=%v status=%d", err, resp3.StatusCode)
	}

	resp4, err := c.Do(jsonReq(t, "GET", srv.URL+"/usage-plans/"+plan.ID, "Bearer admintok", ""))
	if err != nil || resp4.StatusCode != http.StatusOK {
		t.Fatalf("get after patch: err=%v status=%d", err, resp4.StatusCode)
	}
	var after struct {
		RegisterPerMin   *int `json:"register_per_min"`
		RegisterBurst    *int `json:"register_burst"`
		RegisterIPPerMin *int `json:"register_ip_per_min"`
		RegisterIPBurst  *int `json:"register_ip_burst"`
		SubscribePerMin  *int `json:"subscribe_per_min"`
		SubscribeBurst   *int `json:"subscribe_burst"`
	}
	dataOf(t, resp4, &after)
	if after.RegisterPerMin == nil || *after.RegisterPerMin != 99 {
		t.Errorf("after PATCH: want register_per_min=99, got %v", after.RegisterPerMin)
	}
	if after.RegisterBurst == nil || *after.RegisterBurst != 2 {
		t.Errorf("after PATCH: want register_burst=2 (unchanged), got %v", after.RegisterBurst)
	}
	if after.RegisterIPPerMin == nil || *after.RegisterIPPerMin != 3 {
		t.Errorf("after PATCH: want register_ip_per_min=3 (unchanged), got %v", after.RegisterIPPerMin)
	}
	if after.RegisterIPBurst == nil || *after.RegisterIPBurst != 1 {
		t.Errorf("after PATCH: want register_ip_burst=1 (unchanged), got %v", after.RegisterIPBurst)
	}
	if after.SubscribePerMin == nil || *after.SubscribePerMin != 8 {
		t.Errorf("after PATCH: want subscribe_per_min=8 (unchanged), got %v", after.SubscribePerMin)
	}
	if after.SubscribeBurst == nil || *after.SubscribeBurst != 4 {
		t.Errorf("after PATCH: want subscribe_burst=4 (unchanged), got %v", after.SubscribeBurst)
	}
}

func TestUpdateAppPlanValidationBeforeWrite(t *testing.T) {
	srv, _, _, _ := newAdminServer(t)
	c := srv.Client()

	id, _, pat := createAppAdmin(t, srv, `{"name":"atomic"}`)

	resp, err := c.Do(jsonReq(t, "PATCH", srv.URL+"/apps/"+id, "Bearer admintok",
		`{"name":"renamed","usage_plan_id":"nope"}`))
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 for unknown plan, got err=%v status=%d", err, resp.StatusCode)
	}
	resp, err = c.Do(jsonReq(t, "GET", srv.URL+"/apps/"+id, "Bearer "+pat, ""))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("get app: err=%v status=%d", err, resp.StatusCode)
	}
	var a struct {
		Name string `json:"name"`
	}
	dataOf(t, resp, &a)
	if a.Name != "atomic" {
		t.Fatalf("name should be unchanged after failed patch, got %q", a.Name)
	}
}
