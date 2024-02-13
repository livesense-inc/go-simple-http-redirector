package main

import (
	"os"
	"testing"
)

func TestParseCSV(t *testing.T) {
	// Create a temporary CSV file for testing
	tmpfile, err := os.CreateTemp("", "test.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write test data to the temporary CSV file
	csvData := []string{
		"https://before1/1,https://after1/dir/1",
		"https://before1/dir/2,https://after1/dir/dir/2",
		"https://before2/1,https://after2/dir/1",
	}
	for _, row := range csvData {
		_, err := tmpfile.WriteString(row + "\n")
		if err != nil {
			t.Fatal(err)
		}
	}

	// Close the temporary CSV file
	err = tmpfile.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Call the parseCSV function with the temporary CSV file path
	err = parseCSV(tmpfile.Name())
	if err != nil {
		t.Errorf("parseCSV returned an error: %v", err)
	}

	// Check the patterns map
	expected := map[string]redirectPattern{
		"before1/1":     {dest: "https://after1/dir/1"},
		"before1/dir/2": {dest: "https://after1/dir/dir/2"},
		"before2/1":     {dest: "https://after2/dir/1"},
	}
	for k, v := range expected {
		if patterns[k] != v {
			t.Errorf("patterns[%q] = %v, want %v", k, patterns[k], v)
		}
	}
}
