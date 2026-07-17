package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestAPIFactoryContractSweep_AllSpecs(t *testing.T) {
	specs := allFactoryResourceSpecs()
	if len(specs) < 24 {
		t.Fatalf("expected at least 24 factory specs (21 resources + 3 permissions), got %d", len(specs))
	}

	seen := make(map[string]int)
	for _, spec := range specs {
		seen[spec.Name]++
		assertAPIResourceContract(t, spec)
	}

	// Top-level resources appear once; permissions children reuse blueprints/actions/pages names.
	for name, count := range seen {
		switch name {
		case "blueprints", "actions", "pages":
			if count != 2 {
				t.Errorf("%s: expected 2 specs (resource + permissions child), got %d", name, count)
			}
		default:
			if count != 1 {
				t.Errorf("%s: expected 1 spec, got %d", name, count)
			}
		}
	}
}

func TestRegisterAPIRegistersEveryFactorySpec(t *testing.T) {
	root := &cobra.Command{Use: "port"}
	RegisterAPI(root)

	apiCmd, _, err := root.Find([]string{"api"})
	if err != nil || apiCmd == nil {
		t.Fatal("api command not found")
	}

	for _, spec := range allFactoryResourceSpecs() {
		path := []string{spec.Name}
		// Permissions children are nested under api permissions <resource>.
		if isPermissionsChildSpec(spec) {
			path = []string{"permissions", spec.Name}
		}
		resourceCmd, _, findErr := apiCmd.Find(path)
		if findErr != nil || resourceCmd == nil {
			t.Fatalf("api %s not found via path %v: %v", spec.Name, path, findErr)
		}
		for _, op := range spec.Operations {
			sub, _, subErr := resourceCmd.Find([]string{op.Name})
			if subErr != nil || sub == nil {
				t.Fatalf("api %s %s not found", strings.Join(path, " "), op.Name)
			}
		}
	}

	callCmd, _, callErr := apiCmd.Find([]string{"call"})
	if callErr != nil || callCmd == nil {
		t.Fatal("generic api call command not found")
	}
}

func TestAPIFactoryAgentsAndAICustomEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		spec     APIResourceSpec
		opName   string
		args     []string
		data     map[string]interface{}
		method   string
		path     string
		wantBody bool
	}{
		{
			name:     "agents invoke",
			spec:     agentsResourceSpec(),
			opName:   "invoke",
			args:     []string{"agent-1"},
			data:     map[string]interface{}{"prompt": "hello"},
			method:   http.MethodPost,
			path:     "/agent/agent-1/invoke",
			wantBody: true,
		},
		{
			name:     "ai invoke",
			spec:     aiResourceSpec(),
			opName:   "invoke",
			args:     nil,
			data:     map[string]interface{}{"prompt": "summarize"},
			method:   http.MethodPost,
			path:     "/ai/invoke",
			wantBody: true,
		},
		{
			name:     "ai get",
			spec:     aiResourceSpec(),
			opName:   "get",
			args:     []string{"inv-9"},
			method:   http.MethodGet,
			path:     "/ai/invoke/inv-9",
			wantBody: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			var gotBody map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				if r.Body != nil {
					defer r.Body.Close()
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read body: %v", err)
					}
					if len(body) > 0 {
						if err := json.Unmarshal(body, &gotBody); err != nil {
							t.Errorf("decode body: %v", err)
						}
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			t.Cleanup(server.Close)

			client := api.NewClient(api.ClientOpts{
				Token:  &auth.Token{Token: "tok", Claims: auth.Claims{Expiry: time.Now().Add(time.Hour)}},
				APIURL: server.URL,
			})
			defer client.Close()

			op := findOperation(t, tt.spec, tt.opName)
			if op.Run == nil {
				t.Fatal("operation Run is nil")
			}
			_, err := op.Run(context.Background(), client, tt.args, tt.data, APIExtraValues{})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if gotMethod != tt.method {
				t.Errorf("method = %q, want %q", gotMethod, tt.method)
			}
			if gotPath != tt.path {
				t.Errorf("path = %q, want %q", gotPath, tt.path)
			}
			if tt.wantBody {
				if gotBody["prompt"] == nil {
					t.Fatalf("expected prompt in body, got %#v", gotBody)
				}
			} else if len(gotBody) != 0 {
				t.Errorf("expected empty body, got %#v", gotBody)
			}
		})
	}
}

func assertAPIResourceContract(t *testing.T, spec APIResourceSpec) {
	t.Helper()
	if spec.Name == "" {
		t.Fatal("spec.Name is empty")
	}
	if spec.Short == "" {
		t.Fatalf("%s: Short is empty", spec.Name)
	}
	if spec.Singular == "" || spec.Plural == "" {
		t.Fatalf("%s: Singular/Plural must be set", spec.Name)
	}
	if len(spec.Operations) == 0 {
		t.Fatalf("%s: no operations", spec.Name)
	}

	root := &cobra.Command{Use: "port"}
	resourceCmd := registerAPIResource(spec)
	root.AddCommand(resourceCmd)

	opNames := make(map[string]struct{}, len(spec.Operations))
	for _, op := range spec.Operations {
		if _, dup := opNames[op.Name]; dup {
			t.Fatalf("%s: duplicate operation %q", spec.Name, op.Name)
		}
		opNames[op.Name] = struct{}{}
		assertAPIOperationContract(t, spec, resourceCmd, op)
	}
}

func assertAPIOperationContract(t *testing.T, spec APIResourceSpec, resourceCmd *cobra.Command, op APIOperationSpec) {
	t.Helper()
	label := spec.Name + " " + op.Name

	if op.Name == "" || op.Use == "" || op.Short == "" {
		t.Fatalf("%s: Name/Use/Short must be set", label)
	}
	if op.Run == nil {
		t.Fatalf("%s: Run is nil", label)
	}
	if !strings.HasPrefix(op.Use, op.Name) {
		t.Errorf("%s: Use %q should start with Name %q", label, op.Use, op.Name)
	}

	subCmd, _, err := resourceCmd.Find([]string{op.Name})
	if err != nil || subCmd == nil {
		t.Fatalf("%s: command not registered: %v", label, err)
	}
	if subCmd.Use != op.Use {
		t.Errorf("%s: registered Use = %q, want %q", label, subCmd.Use, op.Use)
	}
	if subCmd.Short != op.Short {
		t.Errorf("%s: registered Short = %q, want %q", label, subCmd.Short, op.Short)
	}

	// --org is always present on factory operations.
	if subCmd.Flags().Lookup("org") == nil {
		t.Errorf("%s: missing --org flag", label)
	}

	assertFlagPresence(t, label, subCmd.Flags(), "format", op.HasFormat)
	assertFlagPresence(t, label, subCmd.Flags(), "data", op.DataFile)
	assertFlagPresence(t, label, subCmd.Flags(), "force", op.HasForce)

	if op.DataFile {
		if err := subCmd.ParseFlags([]string{"--data", "payload.json"}); err != nil {
			t.Fatalf("%s: parse --data: %v", label, err)
		}
		dataFile, _ := subCmd.Flags().GetString("data")
		if dataFile != "payload.json" {
			t.Errorf("%s: --data = %q, want payload.json", label, dataFile)
		}
	}
	if op.HasFormat {
		if err := subCmd.ParseFlags([]string{"--format", "yaml"}); err != nil {
			t.Fatalf("%s: parse --format: %v", label, err)
		}
		format, _ := subCmd.Flags().GetString("format")
		if format != "yaml" {
			t.Errorf("%s: --format = %q, want yaml", label, format)
		}
	}
	if op.HasForce {
		if err := subCmd.ParseFlags([]string{"--force"}); err != nil {
			t.Fatalf("%s: parse --force: %v", label, err)
		}
		force, _ := subCmd.Flags().GetBool("force")
		if !force {
			t.Errorf("%s: expected --force true", label)
		}
	}
	if op.ConfirmDelete != op.HasForce {
		// Factory delete confirmations are gated by --force today.
		t.Errorf("%s: ConfirmDelete=%v HasForce=%v (expected equal)", label, op.ConfirmDelete, op.HasForce)
	}

	for _, extra := range op.ExtraFlags {
		flag := subCmd.Flags().Lookup(extra.Name)
		if flag == nil {
			t.Fatalf("%s: missing extra flag --%s", label, extra.Name)
		}
		if extra.Bool {
			if err := subCmd.ParseFlags([]string{"--" + extra.Name + "=false"}); err != nil {
				t.Fatalf("%s: parse --%s: %v", label, extra.Name, err)
			}
			continue
		}
		args := []string{"--" + extra.Name, "value"}
		if err := subCmd.ParseFlags(args); err != nil {
			t.Fatalf("%s: parse --%s: %v", label, extra.Name, err)
		}
		got, _ := subCmd.Flags().GetString(extra.Name)
		if got != "value" {
			t.Errorf("%s: --%s = %q, want value", label, extra.Name, got)
		}
		if extra.Required {
			annotations := flag.Annotations
			if annotations == nil || len(annotations[cobra.BashCompOneRequiredFlag]) == 0 {
				// MarkFlagRequired sets an annotation; also accept Changed-required via Flags().
				if !isRequiredFlag(subCmd, extra.Name) {
					t.Errorf("%s: --%s should be required", label, extra.Name)
				}
			}
		}
	}

	assertArgsContract(t, label, subCmd, op)
}

func assertFlagPresence(t *testing.T, label string, flags *pflag.FlagSet, name string, want bool) {
	t.Helper()
	flag := flags.Lookup(name)
	if want && flag == nil {
		t.Errorf("%s: missing --%s", label, name)
	}
	if !want && flag != nil {
		t.Errorf("%s: unexpected --%s", label, name)
	}
}

func assertArgsContract(t *testing.T, label string, cmd *cobra.Command, op APIOperationSpec) {
	t.Helper()
	if op.Args == nil {
		return
	}

	placeholders := countUsePlaceholders(op.Use)
	good := make([]string, placeholders)
	for i := range good {
		good[i] = "arg"
	}
	if err := cmd.Args(cmd, good); err != nil {
		t.Errorf("%s: Args rejected valid arity %d: %v", label, placeholders, err)
	}
	if placeholders > 0 {
		if err := cmd.Args(cmd, good[:placeholders-1]); err == nil {
			t.Errorf("%s: Args accepted too few arguments", label)
		}
	}
	if err := cmd.Args(cmd, append(good, "extra")); err == nil {
		t.Errorf("%s: Args accepted too many arguments", label)
	}
}

func countUsePlaceholders(use string) int {
	fields := strings.Fields(use)
	n := 0
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "[") && strings.HasSuffix(f, "]") {
			n++
		}
	}
	return n
}

func isRequiredFlag(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		return false
	}
	if ann := flag.Annotations; ann != nil {
		if _, ok := ann[cobra.BashCompOneRequiredFlag]; ok {
			return true
		}
	}
	return false
}

func isPermissionsChildSpec(spec APIResourceSpec) bool {
	if len(spec.Operations) != 2 {
		return false
	}
	hasGet, hasUpdate := false, false
	for _, op := range spec.Operations {
		switch op.Name {
		case "get":
			hasGet = op.HasFormat && !op.DataFile
		case "update":
			hasUpdate = op.DataFile && !op.HasFormat
		}
	}
	return hasGet && hasUpdate && (spec.Name == "blueprints" || spec.Name == "actions" || spec.Name == "pages") &&
		strings.Contains(strings.ToLower(spec.Short), "permission")
}

func findOperation(t *testing.T, spec APIResourceSpec, name string) APIOperationSpec {
	t.Helper()
	for _, op := range spec.Operations {
		if op.Name == name {
			return op
		}
	}
	t.Fatalf("operation %q not found on %s", name, spec.Name)
	return APIOperationSpec{}
}
