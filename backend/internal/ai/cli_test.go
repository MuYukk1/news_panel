package ai

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestHelperProcess is the cross-platform fake CLI. It is a no-op during normal
// test runs and only acts when spawned with GO_WANT_HELPER_PROCESS=1.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	switch os.Getenv("AI_TEST_HELPER_MODE") {
	case "echo":
		fmt.Print(os.Getenv("AI_TEST_STDOUT"))
	case "args":
		fmt.Print(strings.Join(args, "\n"))
	case "sleep":
		ms, _ := strconv.Atoi(os.Getenv("AI_TEST_SLEEP_MS"))
		time.Sleep(time.Duration(ms) * time.Millisecond)
		fmt.Print(os.Getenv("AI_TEST_STDOUT"))
	case "serial":
		path := os.Getenv("AI_TEST_SERIAL_FILE")
		helperAppend(path, "S")
		time.Sleep(30 * time.Millisecond)
		helperAppend(path, "E")
		fmt.Print(`{"domain":"other","score":10,"tags":[],"summary":"x","priority":"low","keep":false}`)
	case "exit":
		fmt.Fprint(os.Stderr, os.Getenv("AI_TEST_STDERR"))
		code, _ := strconv.Atoi(os.Getenv("AI_TEST_EXIT_CODE"))
		os.Exit(code)
	}
	os.Exit(0)
}

func helperAppend(path, s string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		os.Exit(3)
	}
	defer f.Close()
	_, _ = f.WriteString(s)
}

// cliHelper builds a CLIProvider whose command is this test binary re-invoked as
// TestHelperProcess. Behavior is controlled through t.Setenv (inherited by the
// child via exec.CommandContext).
func cliHelper(t *testing.T, mode string, env map[string]string, fakeArgs ...string) *CLIProvider {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("AI_TEST_HELPER_MODE", mode)
	for k, v := range env {
		t.Setenv(k, v)
	}
	template := append([]string{"-test.run=TestHelperProcess", "--"}, fakeArgs...)
	return NewCLIProvider(os.Args[0], template, 5*time.Second)
}

func TestCLIProviderEchoJSON(t *testing.T) {
	p := cliHelper(t, "echo", map[string]string{
		"AI_TEST_STDOUT": `{"domain":"ai","score":80,"tags":["m"],"summary":"摘要","priority":"normal","keep":true}`,
	}, "-p", "{{prompt}}")
	out, err := p.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(out, `"domain":"ai"`) {
		t.Errorf("unexpected output: %q", out)
	}
	if p.Name() != "cli" {
		t.Errorf("name: %q", p.Name())
	}
}

func TestCLIProviderWrappedResult(t *testing.T) {
	inner := `{\"domain\":\"ai\",\"score\":70,\"tags\":[],\"summary\":\"s\",\"priority\":\"normal\",\"keep\":true}`
	p := cliHelper(t, "echo", map[string]string{
		"AI_TEST_STDOUT": `{"type":"result","subtype":"success","result":"` + inner + `"}`,
	}, "{{prompt}}")
	out, err := p.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(out, `"domain":"ai"`) || strings.Contains(out, `"type":"result"`) {
		t.Errorf("wrapper not unwrapped: %q", out)
	}
}

func TestCLIProviderStreamLines(t *testing.T) {
	inner := `{\"domain\":\"deals\",\"score\":60,\"tags\":[],\"summary\":\"s\",\"priority\":\"normal\",\"keep\":true}`
	stdout := `{"type":"system","subtype":"init"}` + "\n" + `{"type":"result","result":"` + inner + `"}`
	p := cliHelper(t, "echo", map[string]string{"AI_TEST_STDOUT": stdout}, "{{prompt}}")
	out, err := p.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(out, `"domain":"deals"`) {
		t.Errorf("stream last-line unwrap failed: %q", out)
	}
}

func TestCLIProviderPromptSubstitution(t *testing.T) {
	p := cliHelper(t, "args", nil, "-p", "{{prompt}}", "--output-format", "json")
	out, err := p.Complete(context.Background(), "SYSTEM", "USER")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(out, "SYSTEM\n\nUSER") {
		t.Errorf("combined prompt not substituted: %q", out)
	}
	if strings.Contains(out, "{{prompt}}") {
		t.Errorf("placeholder left unresolved: %q", out)
	}
	if !strings.Contains(out, "--output-format") {
		t.Errorf("extra args lost: %q", out)
	}
}

func TestCLIProviderPromptAppendedWhenNoPlaceholder(t *testing.T) {
	p := cliHelper(t, "args", nil, "-p")
	out, err := p.Complete(context.Background(), "SYSTEM", "USER")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.HasPrefix(out, "-p\n") || !strings.HasSuffix(out, "SYSTEM\n\nUSER") {
		t.Errorf("prompt should be appended last: %q", out)
	}
}

func TestCLIProviderSerialExecution(t *testing.T) {
	file := t.TempDir() + "/serial.log"
	p := cliHelper(t, "serial", map[string]string{"AI_TEST_SERIAL_FILE": file}, "{{prompt}}")
	const n = 6
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.Complete(context.Background(), "s", "u"); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Complete failed: %v", err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read serial log: %v", err)
	}
	if matched, _ := regexp.MatchString(`^(SE){6}$`, string(data)); !matched {
		t.Errorf("CLI calls were not strictly serial: %q", string(data))
	}
}

func TestCLIProviderTimeout(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("AI_TEST_HELPER_MODE", "sleep")
	t.Setenv("AI_TEST_SLEEP_MS", "3000")
	t.Setenv("AI_TEST_STDOUT", "{}")
	p := NewCLIProvider(os.Args[0], []string{"-test.run=TestHelperProcess", "--", "{{prompt}}"}, 100*time.Millisecond)
	start := time.Now()
	_, err := p.Complete(context.Background(), "s", "u")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should mention timeout: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("timeout not enforced: %v", elapsed)
	}
}

func TestCLIProviderNonZeroExit(t *testing.T) {
	p := cliHelper(t, "exit", map[string]string{"AI_TEST_STDERR": "boom failure", "AI_TEST_EXIT_CODE": "2"}, "{{prompt}}")
	_, err := p.Complete(context.Background(), "s", "u")
	if err == nil {
		t.Fatalf("expected error on non-zero exit")
	}
	if !strings.Contains(err.Error(), "cli failed") {
		t.Errorf("error should wrap cli failure: %v", err)
	}
}

func TestCLIProviderContextCancel(t *testing.T) {
	p := cliHelper(t, "echo", map[string]string{"AI_TEST_STDOUT": "{}"}, "{{prompt}}")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Complete(ctx, "s", "u")
	if err == nil {
		t.Fatalf("cancelled context should error")
	}
}

func TestResolveCLIArgs(t *testing.T) {
	got := resolveCLIArgs([]string{"-p", "{{prompt}}", "--fmt", "json"}, "PROMPT")
	want := []string{"-p", "PROMPT", "--fmt", "json"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d]=%q want %q", i, got[i], want[i])
		}
	}
	got = resolveCLIArgs([]string{"-p"}, "PROMPT")
	if len(got) != 2 || got[1] != "PROMPT" {
		t.Errorf("prompt should append: %#v", got)
	}
}

func TestUnwrapCLIOutput(t *testing.T) {
	triage := `{"domain":"ai","score":80,"keep":true}`
	cases := []struct{ name, in, want string }{
		{"raw triage passthrough", triage, triage},
		{"result string", `{"result":"` + strings.ReplaceAll(triage, `"`, `\"`) + `"}`, triage},
		{"content string", `{"content":"` + strings.ReplaceAll(triage, `"`, `\"`) + `"}`, triage},
		{"message content array", `{"message":{"content":[{"type":"text","text":"` + strings.ReplaceAll(triage, `"`, `\"`) + `"}]}}`, triage},
		{"plain text", "just text", "just text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := unwrapCLIOutput(tc.in); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}