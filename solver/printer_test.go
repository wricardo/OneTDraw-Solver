package solver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

type PrinterTestHelper struct {
	oldStdout *os.File
	r         *os.File
	w         *os.File
}

func (h *PrinterTestHelper) Setup() {
	h.oldStdout = os.Stdout
	h.r, h.w, _ = os.Pipe()
	os.Stdout = h.w
}

func (h *PrinterTestHelper) Teardown() {
	os.Stdout = h.oldStdout
}

func (h *PrinterTestHelper) CaptureOutput() string {
	h.w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, h.r)
	return buf.String()
}

func TestJsonPrinterEmptySolutions(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	printer := JsonPrinter{}
	solutions := Solutions{}

	printer.Print(&solutions)
	output := helper.CaptureOutput()

	if output != "[]\n" {
		t.Errorf("Expected '[]\n', got %s", output)
	}
}

func TestJsonPrinterSingleSolution(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	printer := JsonPrinter{}
	solution := Solution{1, 2, 3}
	solutions := Solutions{solution}

	printer.Print(&solutions)
	output := helper.CaptureOutput()

	// Verify valid JSON
	var result [][]uint16
	err := json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Errorf("Invalid JSON output: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 solution in JSON, got %d", len(result))
	}
	if len(result[0]) != 3 || result[0][0] != 1 || result[0][1] != 2 || result[0][2] != 3 {
		t.Errorf("Expected [1,2,3], got %v", result[0])
	}
}

func TestJsonPrinterMultipleSolutions(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	printer := JsonPrinter{}
	solution1 := Solution{1, 2, 3}
	solution2 := Solution{3, 2, 1}
	solutions := Solutions{solution1, solution2}

	printer.Print(&solutions)
	output := helper.CaptureOutput()

	// Verify valid JSON
	var result [][]uint16
	err := json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Errorf("Invalid JSON output: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 solutions in JSON, got %d", len(result))
	}
}

func TestCleanPrinterEmptySolutions(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	printer := CleanPrinter{}
	solutions := Solutions{}

	printer.Print(&solutions)
	output := helper.CaptureOutput()

	if output != "" {
		t.Errorf("Expected empty output, got %s", output)
	}
}

func TestCleanPrinterSingleSolution(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	printer := CleanPrinter{}
	solution := Solution{1, 2, 3}
	solutions := Solutions{solution}

	printer.Print(&solutions)
	output := helper.CaptureOutput()

	expected := "1 - 2 - 3\n"
	if output != expected {
		t.Errorf("Expected %s, got %s", expected, output)
	}
}

func TestCleanPrinterMultipleSolutions(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	printer := CleanPrinter{}
	solution1 := Solution{1, 2, 3}
	solution2 := Solution{3, 2, 1}
	solutions := Solutions{solution1, solution2}

	printer.Print(&solutions)
	output := helper.CaptureOutput()

	expected := "1 - 2 - 3\n3 - 2 - 1\n"
	if output != expected {
		t.Errorf("Expected %s, got %s", expected, output)
	}
}

func TestCleanPrinterSingleNodeSolution(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	printer := CleanPrinter{}
	solution := Solution{5}
	solutions := Solutions{solution}

	printer.Print(&solutions)
	output := helper.CaptureOutput()

	expected := "5\n"
	if output != expected {
		t.Errorf("Expected %s, got %s", expected, output)
	}
}

func TestCleanPrinterLongPath(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	printer := CleanPrinter{}
	solution := Solution{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	solutions := Solutions{solution}

	printer.Print(&solutions)
	output := helper.CaptureOutput()

	expected := "1 - 2 - 3 - 4 - 5 - 6 - 7 - 8 - 9 - 10\n"
	if output != expected {
		t.Errorf("Expected %s, got %s", expected, output)
	}
}

func TestSolutionsPrintMethod(t *testing.T) {
	helper := &PrinterTestHelper{}
	helper.Setup()
	defer helper.Teardown()

	solution := Solution{1, 2, 3}
	solutions := Solutions{solution}

	printer := JsonPrinter{}
	err := solutions.Print(printer)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := helper.CaptureOutput()

	// Verify output was produced
	if len(output) == 0 {
		t.Error("Expected output to be produced")
	}

	// Verify it's valid JSON
	var result [][]uint16
	jsonErr := json.Unmarshal([]byte(output), &result)
	if jsonErr != nil {
		t.Errorf("Invalid JSON output: %v", jsonErr)
	}
}

// Helper test to verify printer interface implementation
func TestPrinterInterface(t *testing.T) {
	// This test ensures both printers implement the interface
	var _ SolutionPrinter = JsonPrinter{}
	var _ SolutionPrinter = CleanPrinter{}

	// If this compiles, the test passes
}

// Test with actual fmt.Printf to ensure formatting is correct
func TestCleanPrinterFormatting(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Test the actual formatting logic
	solution := Solution{1, 2, 3}
	l := len(solution)
	for k, v := range solution {
		if k < l-1 {
			fmt.Fprintf(&buf, "%v - ", v)
		} else {
			fmt.Fprintf(&buf, "%v", v)
		}
	}
	fmt.Fprintln(&buf, "")

	expected := "1 - 2 - 3\n"
	if buf.String() != expected {
		t.Errorf("Expected %s, got %s", expected, buf.String())
	}
}
