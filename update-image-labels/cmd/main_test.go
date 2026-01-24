package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func TestProcessLines(t *testing.T) {
	cases := []struct {
		Name     string
		Input    []string
		Expected []string
		Labels   map[string]string
	}{
		{
			Name: "no labels",
			Input: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
			},
			Expected: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
			},
			Labels: map[string]string{},
		},

		{
			Name: "add a label",
			Input: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
			},
			Expected: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
				"",
				"LABEL foo=\"bar\"",
			},
			Labels: map[string]string{"foo": "bar"},
		},

		{
			Name: "update a label",
			Input: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
				"",
				"LABEL foo=\"bob\"",
			},
			Expected: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
				"",
				"LABEL foo=\"bar\"",
			},
			Labels: map[string]string{"foo": "bar"},
		},

		{
			Name: "merge labels",
			Input: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
				"",
				"LABEL fizz=\"buzz\"",
			},
			Expected: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
				"",
				"LABEL fizz=\"buzz\"",
				"LABEL foo=\"bar\"",
			},
			Labels: map[string]string{"foo": "bar"},
		},

		{
			Name: "move labels",
			Input: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"LABEL foo2=\"bar\"",
				"RUN bar1",
				"LABEL foo1=\"bar\"",
				"RUN bar2",
			},
			Expected: []string{
				"FROM init",
				"RUN something",
				"LABEL earlier-stage",
				"FROM foo",
				"RUN bar1",
				"RUN bar2",
				"",
				"LABEL foo1=\"bar\"",
				"LABEL foo2=\"bar\"",
				"LABEL foo3=\"bar\"",
			},
			Labels: map[string]string{"foo3": "bar"},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actual := processLines(c.Input, c.Labels)
			if !reflect.DeepEqual(actual, c.Expected) {
				dmp := diffmatchpatch.New()
				diffs := dmp.DiffMain(strings.Join(actual, "\n"), strings.Join(c.Expected, "\n"), true)
				t.Error("failed:\n" + dmp.DiffPrettyText(diffs))
			}
		})
	}
}
