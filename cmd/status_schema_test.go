package cmd

import (
	"encoding/json"
	"reflect"
	"testing"
)

func assertStatusJSONFields(t *testing.T, raw []byte) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "complete", "projects_root", "data_root", "instances", "diagnostics"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("missing envelope field %s", key)
		}
	}
	for _, key := range []string{"instances", "diagnostics"} {
		if string(envelope[key]) == "null" {
			t.Fatalf("null array %s", key)
		}
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(envelope[key], &rows); err != nil {
			t.Fatal(err)
		}
		fields := []string{"owner", "repo", "mode", "pid_file", "state", "pid", "host", "project_path", "port", "uptime_seconds", "uptime_estimated"}
		if key == "diagnostics" {
			fields = []string{"code", "path", "message"}
		}
		for _, row := range rows {
			if key == "instances" {
				var state string
				if err := json.Unmarshal(row["state"], &state); err != nil {
					t.Fatal(err)
				}
				if state == "malformed" || state == "unreadable" {
					if string(row["pid"]) != "null" {
						t.Fatal("invalid PID must be explicit null")
					}
				}
				if state != "live" {
					if string(row["uptime_seconds"]) != "null" || string(row["uptime_estimated"]) != "false" {
						t.Fatal("non-live uptime must be explicit null/false")
					}
				}
				if string(row["host"]) == "null" && (string(row["project_path"]) != "null" || string(row["port"]) != "null") {
					t.Fatal("unresolved project metadata must be explicit null")
				}
			}
			for _, field := range fields {
				if _, ok := row[field]; !ok {
					t.Fatalf("missing %s field %s", key, field)
				}
			}
		}
	}
	var result statusResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	instances := append([]statusInstance{}, result.Instances...)
	diagnostics := append([]statusDiagnostic{}, result.Diagnostics...)
	result.sort()
	if !reflect.DeepEqual(instances, result.Instances) || !reflect.DeepEqual(diagnostics, result.Diagnostics) {
		t.Fatal("unsorted JSON arrays")
	}
}

func TestStatusDiagnosticNullPathSort(t *testing.T) {
	path := ""
	r := emptyStatus()
	r.Diagnostics = []statusDiagnostic{{Code: "a", Path: &path, Message: "first string"}, {Code: "z", Message: "null"}}
	r.sort()
	if r.Diagnostics[0].Path != nil || r.Diagnostics[1].Path == nil {
		t.Fatalf("null must precede even empty string: %+v", r.Diagnostics)
	}
}
