package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultCoverageThreshold = 90.0
	golangCILintVersion      = "v1.64.8"
)

func main() {
	threshold := flag.Float64("min-coverage", defaultCoverageThreshold, "minimum total coverage percentage")
	flag.Parse()

	if err := verify(*threshold); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ verification failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ verification passed")
}

func verify(threshold float64) error {
	files, err := listGoFiles(".")
	if err != nil {
		return fmt.Errorf("discover go files: %w", err)
	}

	if err := checkFormatting(files); err != nil {
		return err
	}

	if err := runLint(); err != nil {
		return err
	}

	coverageFile := filepath.Join(os.TempDir(), fmt.Sprintf("healthd-coverage-%d.out", time.Now().UnixNano()))
	defer os.Remove(coverageFile)

	if err := runTests(coverageFile); err != nil {
		return err
	}

	coverage, err := totalCoverage(coverageFile)
	if err != nil {
		return err
	}

	fmt.Printf("• coverage: %.1f%% (threshold %.1f%%)\n", coverage, threshold)
	if coverage < threshold {
		return fmt.Errorf("coverage %.1f%% is below threshold %.1f%%", coverage, threshold)
	}

	return nil
}

func listGoFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && path != "." {
				return filepath.SkipDir
			}
			if name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) == ".go" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func checkFormatting(files []string) error {
	fmt.Println("• gofmt check")

	if len(files) == 0 {
		fmt.Println("  no Go files found")
		return nil
	}

	args := append([]string{"-l"}, files...)
	cmd := exec.Command("gofmt", args...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("run gofmt -l: %w", err)
	}

	unformatted := strings.TrimSpace(string(out))
	if unformatted == "" {
		fmt.Println("  formatted")
		return nil
	}

	fmt.Printf("  unformatted files:\n%s\n", unformatted)
	return errors.New("gofmt check failed")
}

func runLint() error {
	fmt.Println("• golangci-lint")

	lint := exec.Command("golangci-lint", "run", "./...")
	lint.Stdout = os.Stdout
	lint.Stderr = os.Stderr
	if err := lint.Run(); err == nil {
		return nil
	} else {
		var execErr *exec.Error
		if !errors.As(err, &execErr) {
			return fmt.Errorf("run golangci-lint: %w", err)
		}
	}

	fmt.Printf("  golangci-lint not found in PATH, running go run @%s\n", golangCILintVersion)
	lintViaGo := exec.Command("go", "run", "github.com/golangci/golangci-lint/cmd/golangci-lint@"+golangCILintVersion, "run", "./...")
	lintViaGo.Stdout = os.Stdout
	lintViaGo.Stderr = os.Stderr
	if err := lintViaGo.Run(); err != nil {
		return fmt.Errorf("run golangci-lint via go run: %w", err)
	}

	return nil
}

func runTests(coverageFile string) error {
	pkgs, err := testPackages()
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return errors.New("no test packages found")
	}

	fmt.Printf("• go test %s\n", strings.Join(pkgs, " "))
	args := append([]string{"test"}, pkgs...)
	args = append(args, "-covermode=atomic", "-coverprofile="+coverageFile)
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run go test: %w", err)
	}
	return nil
}

func testPackages() ([]string, error) {
	cmd := exec.Command("go", "list", "./...")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list go packages: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	pkgs := make([]string, 0, len(lines))
	for _, pkg := range lines {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			continue
		}
		if strings.HasSuffix(pkg, "/cmd/verify") {
			continue
		}
		pkgs = append(pkgs, pkg)
	}

	return pkgs, nil
}

func totalCoverage(coverageFile string) (float64, error) {
	cmd := exec.Command("go", "tool", "cover", "-func="+coverageFile)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("compute coverage: %w", err)
	}

	re := regexp.MustCompile(`^total:\s+\(statements\)\s+([0-9.]+)%$`)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "total:") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) != 2 {
			return 0, fmt.Errorf("unexpected total coverage format: %q", line)
		}

		var value float64
		if _, err := fmt.Sscanf(matches[1], "%f", &value); err != nil {
			return 0, fmt.Errorf("parse total coverage: %w", err)
		}
		return value, nil
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read coverage output: %w", err)
	}

	return 0, errors.New("total coverage line not found")
}
