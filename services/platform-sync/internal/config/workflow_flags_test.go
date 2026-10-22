package config

import "testing"

func TestEnvFlag(t *testing.T) {
	for v, want := range map[string]bool{
		"true": true, "1": true,
		"": false, "false": false, "0": false, "TRUE": false, "yes": false, "on": false, " true": false,
	} {
		if got := EnvFlag(v); got != want {
			t.Errorf("EnvFlag(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestWorkflowFlagsDefaultOff(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("CREDENTIAL_ENCRYPTION_KEY", devCredKey)
	t.Setenv("ANI_ENCRYPTION_KEY", devCredKey)
	t.Setenv("WORKFLOW_ACTIONS_ENABLED", "")
	t.Setenv("WORKFLOW_RINGBA_PAUSE_VERIFIED", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkflowActionsEnabled || cfg.WorkflowRingbaPauseVerified {
		t.Fatal("workflow switches must default off")
	}
	t.Setenv("WORKFLOW_ACTIONS_ENABLED", "true")
	t.Setenv("WORKFLOW_RINGBA_PAUSE_VERIFIED", "1")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.WorkflowActionsEnabled || !cfg.WorkflowRingbaPauseVerified {
		t.Fatal("true / 1 must enable")
	}
	t.Setenv("WORKFLOW_ACTIONS_ENABLED", "yes")
	cfg, _ = Load()
	if cfg.WorkflowActionsEnabled {
		t.Fatal("only true/1 enable")
	}
}
