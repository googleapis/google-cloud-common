// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"go/doc"
	"log"
	"os"
	"path/filepath"
	"strings"

	tpb "github.com/GoogleCloudPlatform/google-cloud-common/testing/firestore/genproto"
	"github.com/golang/protobuf/proto"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	fspb "google.golang.org/genproto/googleapis/firestore/v1beta1"
)

const (
	database = "projects/projectID/databases/(default)"
	collPath = database + "/documents/C"
	docPath  = collPath + "/d"
)

var outputDir = flag.String("o", "", "directory to write test files")

var (
	updateTimePrecondition = &fspb.Precondition{
		ConditionType: &fspb.Precondition_UpdateTime{&tspb.Timestamp{Seconds: 42}},
	}

	existsTruePrecondition = &fspb.Precondition{
		ConditionType: &fspb.Precondition_Exists{true},
	}

	nTests int
)

// A writeTest describes a Create, Set, Update or UpdatePaths call.
type writeTest struct {
	suffix           string             // textproto filename suffix
	desc             string             // short description
	comment          string             // detailed explanation (comment in textproto file)
	commentForUpdate string             // additional comment for update operations.
	inData           string             // input data, as JSON
	paths            [][]string         // fields paths for UpdatePaths
	values           []string           // values for UpdatePaths, as JSON
	opt              *tpb.SetOption     // option for Set
	precond          *fspb.Precondition // precondition for Update

	outData       map[string]*fspb.Value // expected data in update write
	mask          []string               // expected fields in update mask
	maskForUpdate []string               // mask, but only for Update/UpdatePaths
	transform     []string               // expected fields in transform
	isErr         bool                   // arguments result in a client-side error
}

var (
	basicTests = []writeTest{
		{
			suffix:        "basic",
			desc:          "basic",
			comment:       `A simple call, resulting in a single update operation.`,
			inData:        `{"a": 1}`,
			paths:         [][]string{{"a"}},
			values:        []string{`1`},
			maskForUpdate: []string{"a"},
			outData:       mp("a", 1),
		},
		{
			suffix:        "complex",
			desc:          "complex",
			comment:       `A call to a write method with complicated input data.`,
			inData:        `{"a": [1, 2.5], "b": {"c": ["three", {"d": true}]}}`,
			paths:         [][]string{{"a"}, {"b"}},
			values:        []string{`[1, 2.5]`, `{"c": ["three", {"d": true}]}`},
			maskForUpdate: []string{"a", "b"},
			outData: mp(
				"a", []interface{}{1, 2.5},
				"b", mp("c", []interface{}{"three", mp("d", true)}),
			),
		},
	}

	// tests for Create and Set
	createSetTests = []writeTest{
		{
			suffix:  "empty",
			desc:    "creating or setting an empty map",
			inData:  `{}`,
			outData: mp(),
		},
		{
			suffix:  "nosplit",
			desc:    "don’t split on dots", // go/set-update #1
			comment: `Create and Set treat their map keys literally. They do not split on dots.`,
			inData:  `{ "a.b": { "c.d": 1 }, "e": 2 }`,
			outData: mp("a.b", mp("c.d", 1), "e", 2),
		},
		{
			suffix:  "special-chars",
			desc:    "non-alpha characters in map keys",
			comment: `Create and Set treat their map keys literally. They do not escape special characters.`,
			inData:  `{ "*": { ".": 1 }, "~": 2 }`,
			outData: mp("*", mp(".", 1), "~", 2),
		},
		{
			suffix:  "nodel",
			desc:    "Delete cannot appear in data",
			comment: `The Delete sentinel cannot be used in Create, or in Set without a Merge option.`,
			inData:  `{"a": 1, "b": "Delete"}`,
			isErr:   true,
		},
	}

	// tests for Update and UpdatePaths
	updateTests = []writeTest{
		{
			suffix: "del",
			desc:   "Delete",
			comment: `If a field's value is the Delete sentinel, then it doesn't appear
in the update data, but does in the mask.`,
			inData:  `{"a": 1, "b": "Delete"}`,
			paths:   [][]string{{"a"}, {"b"}},
			values:  []string{`1`, `"Delete"`},
			outData: mp("a", 1),
			mask:    []string{"a", "b"},
		},
		{
			suffix: "del-alone",
			desc:   "Delete alone",
			comment: `If the input data consists solely of Deletes, then the update
operation has no map, just an update mask.`,
			inData:  `{"a": "Delete"}`,
			paths:   [][]string{{"a"}},
			values:  []string{`"Delete"`},
			outData: nil,
			mask:    []string{"a"},
		},
		{
			suffix:  "uptime",
			desc:    "last-update-time precondition",
			comment: `The Update call supports a last-update-time precondition.`,
			inData:  `{"a": 1}`,
			paths:   [][]string{{"a"}},
			values:  []string{`1`},
			precond: updateTimePrecondition,
			outData: mp("a", 1),
			mask:    []string{"a"},
		},
		{
			suffix:  "no-paths",
			desc:    "no paths",
			comment: `It is a client-side error to call Update with empty data.`,
			inData:  `{}`,
			paths:   nil,
			values:  nil,
			isErr:   true,
		},
		{
			suffix:  "fp-empty-component",
			desc:    "empty field path component",
			comment: `Empty fields are not allowed.`,
			inData:  `{"a..b": 1}`,
			paths:   [][]string{{"*", ""}},
			values:  []string{`1`},
			isErr:   true,
		},
		{
			suffix:  "prefix-1",
			desc:    "prefix #1",
			comment: `In the input data, one field cannot be a prefix of another.`,
			inData:  `{"a.b": 1, "a": 2}`,
			paths:   [][]string{{"a", "b"}, {"a"}},
			values:  []string{`1`, `2`},
			isErr:   true,
		},
		{
			suffix:  "prefix-2",
			desc:    "prefix #2",
			comment: `In the input data, one field cannot be a prefix of another.`,
			inData:  `{"a": 1, "a.b": 2}`,
			paths:   [][]string{{"a"}, {"a", "b"}},
			values:  []string{`1`, `2`},
			isErr:   true,
		},
		{
			suffix:  "prefix-3",
			desc:    "prefix #3",
			comment: `In the input data, one field cannot be a prefix of another, even if the values could in principle be combined.`,
			inData:  `{"a": {"b": 1}, "a.d": 2}`,
			paths:   [][]string{{"a"}, {"a", "d"}},
			values:  []string{`{"b": 1}`, `2`},
			isErr:   true,
		},
		{
			suffix:  "del-nested",
			desc:    "Delete cannot be nested",
			comment: `The Delete sentinel must be the value of a top-level key.`,
			inData:  `{"a": {"b": "Delete"}}`,
			paths:   [][]string{{"a"}},
			values:  []string{`{"b": "Delete"}`},
			isErr:   true,
		},
		{
			suffix:  "exists-precond",
			desc:    "Exists precondition is invalid",
			comment: `The Update method does not support an explicit exists precondition.`,
			inData:  `{"a": 1}`,
			paths:   [][]string{{"a"}},
			values:  []string{`1`},
			precond: existsTruePrecondition,
			isErr:   true,
		},
		{
			suffix: "st-alone",
			desc:   "ServerTimestamp alone",
			comment: `If the only values in the input are ServerTimestamps, then no
update operation should be produced.`,
			inData:        `{"a": "ServerTimestamp"}`,
			paths:         [][]string{{"a"}},
			values:        []string{`"ServerTimestamp"`},
			outData:       nil,
			maskForUpdate: nil,
			transform:     []string{"a"},
		},
	}

	serverTimestampTests = []writeTest{
		{
			suffix: "st",
			desc:   "ServerTimestamp with data",
			comment: `A key with the special ServerTimestamp sentinel is removed from
the data in the update operation. Instead it appears in a separate Transform operation.
Note that in these tests, the string "ServerTimestamp" should be replaced with the
special ServerTimestamp value.`,
			inData:        `{"a": 1, "b": "ServerTimestamp"}`,
			paths:         [][]string{{"a"}, {"b"}},
			values:        []string{`1`, `"ServerTimestamp"`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a"},
			transform:     []string{"b"},
		},
		{
			suffix: "st-nested",
			desc:   "nested ServerTimestamp field",
			comment: `A ServerTimestamp value can occur at any depth. In this case,
the transform applies to the field path "b.c". Since "c" is removed from the update,
"b" becomes empty, so it is also removed from the update.`,
			inData:        `{"a": 1, "b": {"c": "ServerTimestamp"}}`,
			paths:         [][]string{{"a"}, {"b"}},
			values:        []string{`1`, `{"c": "ServerTimestamp"}`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a", "b"},
			transform:     []string{"b.c"},
		},
		{
			suffix: "st-multi",
			desc:   "multiple ServerTimestamp fields",
			comment: `A document can have more than one ServerTimestamp field.
Since all the ServerTimestamp fields are removed, the only field in the update is "a".`,
			commentForUpdate: `b is not in the mask because it will be set in the transform.
c must be in the mask: it should be replaced entirely. The transform will set c.d to the
timestamp, but the update will delete the rest of c.`,
			inData:        `{"a": 1, "b": "ServerTimestamp", "c": {"d": "ServerTimestamp"}}`,
			paths:         [][]string{{"a"}, {"b"}, {"c"}},
			values:        []string{`1`, `"ServerTimestamp"`, `{"d": "ServerTimestamp"}`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a", "c"},
			transform:     []string{"b", "c.d"},
		},
	}

	// Common errors with the ServerTimestamp and Delete sentinels.
	sentinelErrorTests = []writeTest{
		{
			suffix: "st-noarray",
			desc:   "ServerTimestamp cannot be in an array value",
			comment: `The ServerTimestamp sentinel must be the value of a field. Firestore
transforms don't support array indexing.`,
			inData: `{"a": [1, 2, "ServerTimestamp"]}`,
			paths:  [][]string{{"a"}},
			values: []string{`[1, 2, "ServerTimestamp"]`},
			isErr:  true,
		},
		{
			suffix: "st-noarray-nested",
			desc:   "ServerTimestamp cannot be anywhere inside an array value",
			comment: `There cannot be an array value anywhere on the path from the document
root to the ServerTimestamp sentinel. Firestore transforms don't support array indexing.`,
			inData: `{"a": [1, {"b": "ServerTimestamp"}]}`,
			paths:  [][]string{{"a"}},
			values: []string{`[1, {"b": "ServerTimestamp"}]`},
			isErr:  true,
		},
		{
			suffix: "del-noarray",
			desc:   "Delete cannot be in an array value",
			comment: `The Delete sentinel must be the value of a field. Deletes are
implemented by turning the path to the Delete sentinel into a FieldPath, and FieldPaths
do not support array indexing.`,
			inData: `{"a": [1, 2, "Delete"]}`,
			paths:  [][]string{{"a"}},
			values: []string{`[1, 2, "Delete"]`},
			isErr:  true,
		},
		{
			suffix: "del-noarray-nested",
			desc:   "Delete cannot be anywhere inside an array value",
			comment: `The Delete sentinel must be the value of a field. Deletes are implemented
by turning the path to the Delete sentinel into a FieldPath, and FieldPaths do not support
array indexing.`,
			inData: `{"a": [1, {"b": "Delete"}]}`,
			paths:  [][]string{{"a"}},
			values: []string{`[1, {"b": "Delete"}]`},
			isErr:  true,
		},
	}
)

func main() {
	flag.Parse()
	if *outputDir == "" {
		log.Fatal("-o required")
	}
	suite := &tpb.TestSuite{}
	genGet(suite)
	genCreate(suite)
	genSet(suite)
	genUpdate(suite)
	genUpdatePaths(suite)
	genDelete(suite)
	genQuery(suite)
	if err := writeProtoToFile(filepath.Join(*outputDir, "test-suite.binproto"), suite); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %d tests to %s\n", nTests, *outputDir)
}

func genGet(suite *tpb.TestSuite) {
	tp := &tpb.Test{
		Description: "get: get a document",
		Test: &tpb.Test_Get{&tpb.GetTest{
			DocRefPath: docPath,
			Request:    &fspb.GetDocumentRequest{Name: docPath},
		}},
	}
	suite.Tests = append(suite.Tests, tp)
	outputTestText("get-basic", "A call to DocumentRef.Get.", tp)
}

func genCreate(suite *tpb.TestSuite) {
	var tests []writeTest
	tests = append(tests, basicTests...)
	tests = append(tests, createSetTests...)
	tests = append(tests, serverTimestampTests...)
	tests = append(tests, sentinelErrorTests...)
	tests = append(tests, writeTest{
		suffix: "st-alone",
		desc:   "ServerTimestamp alone",
		comment: `If the only values in the input are ServerTimestamps, then no
update operation should be produced.`,
		inData:        `{"a": "ServerTimestamp"}`,
		paths:         [][]string{{"a"}},
		values:        []string{`"ServerTimestamp"`},
		outData:       nil,
		maskForUpdate: nil,
		transform:     []string{"a"},
	})

	precond := &fspb.Precondition{
		ConditionType: &fspb.Precondition_Exists{false},
	}
	for _, test := range tests {
		var req *fspb.CommitRequest
		if !test.isErr {
			req = newCommitRequest(test.outData, test.mask, precond, test.transform)
		}
		tp := &tpb.Test{
			Description: "create: " + test.desc,
			Test: &tpb.Test_Create{&tpb.CreateTest{
				DocRefPath: docPath,
				JsonData:   test.inData,
				Request:    req,
				IsError:    test.isErr,
			}},
		}
		suite.Tests = append(suite.Tests, tp)
		outputTestText(fmt.Sprintf("create-%s", test.suffix), test.comment, tp)
	}

}
func genSet(suite *tpb.TestSuite) {
	var tests []writeTest
	tests = append(tests, basicTests...)
	tests = append(tests, createSetTests...)
	tests = append(tests, serverTimestampTests...)
	tests = append(tests, sentinelErrorTests...)
	tests = append(tests, []writeTest{
		{
			suffix: "st-alone",
			desc:   "ServerTimestamp alone",
			comment: `If the only values in the input are ServerTimestamps, then
an update operation with an empty map should be produced.`,
			inData:        `{"a": "ServerTimestamp"}`,
			paths:         [][]string{{"a"}},
			values:        []string{`"ServerTimestamp"`},
			outData:       mp(),
			maskForUpdate: nil,
			transform:     []string{"a"},
		},
		{
			suffix:  "mergeall",
			desc:    "MergeAll",
			comment: "The MergeAll option with a simple piece of data.",
			inData:  `{"a": 1, "b": 2}`,
			opt:     mergeAllOption,
			outData: mp("a", 1, "b", 2),
			mask:    []string{"a", "b"},
		},
		{
			suffix: "mergeall-nested", // go/set-update #3
			desc:   "MergeAll with nested fields",
			comment: `MergeAll with nested fields results in an update mask that
includes entries for all the leaf fields.`,
			inData:  `{"h": { "g": 3, "f": 4 }}`,
			opt:     mergeAllOption,
			outData: mp("h", mp("g", 3, "f", 4)),
			mask:    []string{"h.f", "h.g"},
		},
		{
			suffix:  "merge",
			desc:    "Merge with a field",
			comment: `Fields in the input data but not in a merge option are pruned.`,
			inData:  `{"a": 1, "b": 2}`,
			opt:     mergeOption([]string{"a"}),
			outData: mp("a", 1),
			mask:    []string{"a"},
		},
		{
			suffix: "merge-nested", // go/set-update #4
			desc:   "Merge with a nested field",
			comment: `A merge option where the field is not at top level.
Only fields mentioned in the option are present in the update operation.`,
			inData:  `{"h": {"g": 4, "f": 5}}`,
			opt:     mergeOption([]string{"h", "g"}),
			outData: mp("h", mp("g", 4)),
			mask:    []string{"h.g"},
		},
		{
			suffix: "merge-nonleaf", // go/set-update #5
			desc:   "Merge field is not a leaf",
			comment: `If a field path is in a merge option, the value at that path
replaces the stored value. That is true even if the value is complex.`,
			inData:  `{"h": {"f": 5, "g": 6}, "e": 7}`,
			opt:     mergeOption([]string{"h"}),
			outData: mp("h", mp("f", 5, "g", 6)),
			mask:    []string{"h"},
		},
		{
			suffix:  "merge-fp",
			desc:    "Merge with FieldPaths",
			comment: `A merge with fields that use special characters.`,
			inData:  `{"*": {"~": true}}`,
			opt:     mergeOption([]string{"*", "~"}),
			outData: mp("*", mp("~", true)),
			mask:    []string{"`*`.`~`"},
		},
		{
			suffix: "st-mergeall",
			desc:   "ServerTimestamp with MergeAll",
			comment: `Just as when no merge option is specified, ServerTimestamp
sentinel values are removed from the data in the update operation and become
transforms.`,
			inData:    `{"a": 1, "b": "ServerTimestamp"}`,
			opt:       mergeAllOption,
			outData:   mp("a", 1),
			mask:      []string{"a"},
			transform: []string{"b"},
		},
		{
			suffix: "st-alone-mergeall",
			desc:   "ServerTimestamp alone with MergeAll",
			comment: `If the only values in the input are ServerTimestamps, then no
update operation should be produced.`,
			inData:        `{"a": "ServerTimestamp"}`,
			opt:           mergeAllOption,
			paths:         [][]string{{"a"}},
			values:        []string{`"ServerTimestamp"`},
			outData:       nil,
			maskForUpdate: nil,
			transform:     []string{"a"},
		},
		{
			suffix: "st-merge-both",
			desc:   "ServerTimestamp with Merge of both fields",
			inData: `{"a": 1, "b": "ServerTimestamp"}`,
			comment: `Just as when no merge option is specified, ServerTimestamp
sentinel values are removed from the data in the update operation and become
transforms.`,
			opt:       mergeOption([]string{"a"}, []string{"b"}),
			outData:   mp("a", 1),
			mask:      []string{"a"},
			transform: []string{"b"},
		},
		{
			suffix: "st-nomerge",
			desc:   "If is ServerTimestamp not in Merge, no transform",
			comment: `If the ServerTimestamp value is not mentioned in a merge option,
then it is pruned from the data but does not result in a transform.`,
			inData:  `{"a": 1, "b": "ServerTimestamp"}`,
			opt:     mergeOption([]string{"a"}),
			outData: mp("a", 1),
			mask:    []string{"a"},
		},
		{
			suffix: "st-merge-nowrite",
			desc:   "If no ordinary values in Merge, no write",
			comment: `If all the fields in the merge option have ServerTimestamp
values, then no update operation is produced, only a transform.`,
			inData:    `{"a": 1, "b": "ServerTimestamp"}`,
			opt:       mergeOption([]string{"b"}),
			transform: []string{"b"},
		},
		{
			suffix: "st-merge-nonleaf",
			desc:   "non-leaf merge field with ServerTimestamp",
			comment: `If a field path is in a merge option, the value at that path
replaces the stored value, and ServerTimestamps inside that value become transforms
as usual.`,
			inData:    `{"h": {"f": 5, "g": "ServerTimestamp"}, "e": 7}`,
			opt:       mergeOption([]string{"h"}),
			outData:   mp("h", mp("f", 5)),
			mask:      []string{"h"},
			transform: []string{"h.g"},
		},
		{
			suffix: "st-merge-nonleaf-alone",
			desc:   "non-leaf merge field with ServerTimestamp alone",
			comment: `If a field path is in a merge option, the value at that path
replaces the stored value. If the value has only ServerTimestamps, they become transforms
and we clear the value by including the field path in the update mask.`,
			inData:    `{"h": {"g": "ServerTimestamp"}, "e": 7}`,
			opt:       mergeOption([]string{"h"}),
			mask:      []string{"h"},
			transform: []string{"h.g"},
		},
		{
			suffix:  "del-mergeall",
			desc:    "Delete with MergeAll",
			comment: "A Delete sentinel can appear with a mergeAll option.",
			inData:  `{"a": 1, "b": {"c": "Delete"}}`,
			opt:     mergeAllOption,
			outData: mp("a", 1),
			mask:    []string{"a", "b.c"},
		},
		{
			suffix:  "del-merge",
			desc:    "Delete with merge",
			comment: "A Delete sentinel can appear with a merge option.",
			inData:  `{"a": 1, "b": {"c": "Delete"}}`,
			opt:     mergeOption([]string{"a"}, []string{"b", "c"}),
			outData: mp("a", 1),
			mask:    []string{"a", "b.c"},
		},
		{
			suffix: "del-merge-alone",
			desc:   "Delete with merge",
			comment: `A Delete sentinel can appear with a merge option. If the delete
paths are the only ones to be merged, then no document is sent, just an update mask.`,
			inData:  `{"a": 1, "b": {"c": "Delete"}}`,
			opt:     mergeOption([]string{"b", "c"}),
			outData: nil,
			mask:    []string{"b.c"},
		},
		{
			suffix:  "mergeall-empty",
			desc:    "MergeAll can be specified with empty data.",
			comment: `This is a valid call that can be used to ensure a document exists.`,
			inData:  `{}`,
			opt:     mergeAllOption,
			outData: mp(),
		},
		// Errors:
		{
			suffix: "merge-present",
			desc:   "Merge fields must all be present in data",
			comment: `The client signals an error if a merge option mentions a path
that is not in the input data.`,
			inData: `{"a": 1}`,
			opt:    mergeOption([]string{"b"}, []string{"a"}),
			isErr:  true,
		},
		{
			suffix: "del-wo-merge",
			desc:   "Delete cannot appear unless a merge option is specified",
			comment: `Without a merge option, Set replaces the document with the input
data. A Delete sentinel in the data makes no sense in this case.`,
			inData: `{"a": 1, "b": "Delete"}`,
			isErr:  true,
		},
		{
			suffix: "del-nomerge",
			desc:   "Delete cannot appear in an unmerged field",
			comment: `The client signals an error if the Delete sentinel is in the
input data, but not selected by a merge option, because this is most likely a programming
bug.`,
			inData: `{"a": 1, "b": "Delete"}`,
			opt:    mergeOption([]string{"a"}),
			isErr:  true,
		},
		{
			suffix: "del-nonleaf",
			desc:   "Delete cannot appear as part of a merge path",
			comment: `If a Delete is part of the value at a merge path, then the user is
confused: their merge path says "replace this entire value" but their Delete says
"delete this part of the value". This should be an error, just as if they specified Delete
in a Set with no merge.`,
			inData: `{"h": {"g": "Delete"}}`,
			opt:    mergeOption([]string{"h"}),
			isErr:  true,
		},
		{
			suffix: "merge-prefix",
			desc:   "One merge path cannot be the prefix of another",
			comment: `The prefix would make the other path meaningless, so this is
probably a programming error.`,
			inData: `{"a": {"b": 1}}`,
			opt:    mergeOption([]string{"a"}, []string{"a", "b"}),
			isErr:  true,
		},
	}...)

	for _, test := range tests {
		var req *fspb.CommitRequest
		if !test.isErr {
			req = newCommitRequest(test.outData, test.mask, nil, test.transform)
		}
		prefix := "set"
		if test.opt != nil && !test.opt.All {
			prefix = "set-merge"
		}
		tp := &tpb.Test{
			Description: prefix + ": " + test.desc,
			Test: &tpb.Test_Set{&tpb.SetTest{
				DocRefPath: docPath,
				Option:     test.opt,
				JsonData:   test.inData,
				Request:    req,
				IsError:    test.isErr,
			}},
		}
		suite.Tests = append(suite.Tests, tp)
		outputTestText(fmt.Sprintf("set-%s", test.suffix), test.comment, tp)
	}
}

func genUpdate(suite *tpb.TestSuite) {
	var tests []writeTest
	tests = append(tests, basicTests...)
	tests = append(tests, updateTests...)
	tests = append(tests, serverTimestampTests...)
	tests = append(tests, sentinelErrorTests...)
	tests = append(tests, []writeTest{
		{
			suffix:  "split",
			desc:    "split on dots",
			comment: `The Update method splits top-level keys at dots.`,
			inData:  `{"a.b.c": 1}`,
			outData: mp("a", mp("b", mp("c", 1))),
			mask:    []string{"a.b.c"},
		},
		{
			suffix:  "quoting",
			desc:    "non-letter starting chars are quoted, except underscore",
			comment: `In a field path, any component beginning with a non-letter or underscore is quoted.`,
			inData:  `{"_0.1.+2": 1}`,
			outData: mp("_0", mp("1", mp("+2", 1))),
			mask:    []string{"_0.`1`.`+2`"},
		},
		{
			suffix: "split-top-level", // go/set-update #6
			desc:   "Split on dots for top-level keys only",
			comment: `The Update method splits only top-level keys at dots. Keys at
other levels are taken literally.`,
			inData:  `{"h.g": {"j.k": 6}}`,
			outData: mp("h", mp("g", mp("j.k", 6))),
			mask:    []string{"h.g"},
		},
		{
			suffix: "del-dot",
			desc:   "Delete with a dotted field",
			comment: `After expanding top-level dotted fields, fields with Delete
values are pruned from the output data, but appear in the update mask.`,
			inData:  `{"a": 1, "b.c": "Delete", "b.d": 2}`,
			outData: mp("a", 1, "b", mp("d", 2)),
			mask:    []string{"a", "b.c", "b.d"},
		},

		{
			suffix: "st-dot",
			desc:   "ServerTimestamp with dotted field",
			comment: `Like other uses of ServerTimestamp, the data is pruned and the
field does not appear in the update mask, because it is in the transform. In this case
An update operation is produced just to hold the precondition.`,
			inData:    `{"a.b.c": "ServerTimestamp"}`,
			transform: []string{"a.b.c"},
		},
		// Errors
		{
			suffix:  "badchar",
			desc:    "invalid character",
			comment: `The keys of the data given to Update are interpreted, unlike those of Create and Set. They cannot contain special characters.`,
			inData:  `{"a~b": 1}`,
			isErr:   true,
		},
	}...)

	for _, test := range tests {
		tp := &tpb.Test{
			Description: "update: " + test.desc,
			Test: &tpb.Test_Update{&tpb.UpdateTest{
				DocRefPath:   docPath,
				Precondition: test.precond,
				JsonData:     test.inData,
				Request:      newUpdateCommitRequest(test),
				IsError:      test.isErr,
			}},
		}
		comment := test.comment
		if test.commentForUpdate != "" {
			comment += "\n\n" + test.commentForUpdate
		}
		suite.Tests = append(suite.Tests, tp)
		outputTestText(fmt.Sprintf("update-%s", test.suffix), comment, tp)
	}
}

func genUpdatePaths(suite *tpb.TestSuite) {
	var tests []writeTest
	tests = append(tests, basicTests...)
	tests = append(tests, updateTests...)
	tests = append(tests, serverTimestampTests...)
	tests = append(tests, sentinelErrorTests...)
	tests = append(tests, []writeTest{
		{
			suffix: "fp-multi",
			desc:   "multiple-element field path",
			comment: `The UpdatePaths or equivalent method takes a list of FieldPaths.
Each FieldPath is a sequence of uninterpreted path components.`,
			paths:   [][]string{{"a", "b"}},
			values:  []string{`1`},
			outData: mp("a", mp("b", 1)),
			mask:    []string{"a.b"},
		},
		{
			suffix:  "fp-nosplit", // go/set-update #7, approx.
			desc:    "FieldPath elements are not split on dots",
			comment: `FieldPath components are not split on dots.`,
			paths:   [][]string{{"a.b", "f.g"}},
			values:  []string{`{"n.o": 7}`},
			outData: mp("a.b", mp("f.g", mp("n.o", 7))),
			mask:    []string{"`a.b`.`f.g`"},
		},
		{
			suffix:  "special-chars",
			desc:    "special characters",
			comment: `FieldPaths can contain special characters.`,
			paths:   [][]string{{"*", "~"}, {"*", "`"}},
			values:  []string{`1`, `2`},
			outData: mp("*", mp("~", 1, "`", 2)),
			mask:    []string{"`*`.`\\``", "`*`.`~`"},
		},
		{
			suffix:  "fp-del", // see https://github.com/googleapis/nodejs-firestore/pull/119
			desc:    "field paths with delete",
			comment: `If one nested field is deleted, and another isn't, preserve the second.`,
			paths:   [][]string{{"foo", "bar"}, {"foo", "delete"}},
			values:  []string{`1`, `"Delete"`},
			outData: mp("foo", mp("bar", 1)),
			mask:    []string{"foo.bar", "foo.delete"},
		},
		// Errors
		{
			suffix:  "fp-empty",
			desc:    "empty field path",
			comment: `A FieldPath of length zero is invalid.`,
			paths:   [][]string{{}},
			values:  []string{`1`},
			isErr:   true,
		},
		{
			suffix:  "fp-dup",
			desc:    "duplicate field path",
			comment: `The same field cannot occur more than once.`,
			paths:   [][]string{{"a"}, {"b"}, {"a"}},
			values:  []string{`1`, `2`, `3`},
			isErr:   true,
		},
	}...)

	for _, test := range tests {
		if len(test.paths) != len(test.values) {
			log.Fatalf("test %s has mismatched paths and values", test.desc)
		}
		tp := &tpb.Test{
			Description: "update-paths: " + test.desc,
			Test: &tpb.Test_UpdatePaths{&tpb.UpdatePathsTest{
				DocRefPath:   docPath,
				Precondition: test.precond,
				FieldPaths:   toFieldPaths(test.paths),
				JsonValues:   test.values,
				Request:      newUpdateCommitRequest(test),
				IsError:      test.isErr,
			}},
		}
		comment := test.comment
		if test.commentForUpdate != "" {
			comment += "\n\n" + test.commentForUpdate
		}
		suite.Tests = append(suite.Tests, tp)
		outputTestText(fmt.Sprintf("update-paths-%s", test.suffix), test.comment, tp)
	}
}

func genDelete(suite *tpb.TestSuite) {
	for _, test := range []struct {
		suffix  string
		desc    string
		comment string
		precond *fspb.Precondition
		isErr   bool
	}{
		{
			suffix:  "no-precond",
			desc:    "delete without precondition",
			comment: `An ordinary Delete call.`,
			precond: nil,
		},
		{
			suffix:  "time-precond",
			desc:    "delete with last-update-time precondition",
			comment: `Delete supports a last-update-time precondition.`,
			precond: updateTimePrecondition,
		},
		{
			suffix:  "exists-precond",
			desc:    "delete with exists precondition",
			comment: `Delete supports an exists precondition.`,
			precond: existsTruePrecondition,
		},
	} {
		var req *fspb.CommitRequest
		if !test.isErr {
			req = &fspb.CommitRequest{
				Database: database,
				Writes:   []*fspb.Write{{Operation: &fspb.Write_Delete{docPath}}},
			}
			if test.precond != nil {
				req.Writes[0].CurrentDocument = test.precond
			}
		}
		tp := &tpb.Test{
			Description: "delete: " + test.desc,
			Test: &tpb.Test_Delete{&tpb.DeleteTest{
				DocRefPath:   docPath,
				Precondition: test.precond,
				Request:      req,
				IsError:      test.isErr,
			}},
		}
		suite.Tests = append(suite.Tests, tp)
		outputTestText(fmt.Sprintf("delete-%s", test.suffix), test.comment, tp)
	}
}

func newUpdateCommitRequest(test writeTest) *fspb.CommitRequest {
	if test.isErr {
		return nil
	}
	mask := test.mask
	if mask == nil {
		mask = test.maskForUpdate
	} else if test.maskForUpdate != nil {
		log.Fatalf("test %s has mask and maskForUpdate", test.desc)
	}
	precond := test.precond
	if precond == nil {
		precond = existsTruePrecondition
	}
	return newCommitRequest(test.outData, mask, precond, test.transform)
}

func newCommitRequest(writeFields map[string]*fspb.Value, mask []string, precond *fspb.Precondition, transform []string) *fspb.CommitRequest {
	var writes []*fspb.Write
	if writeFields != nil || mask != nil {
		w := &fspb.Write{
			Operation: &fspb.Write_Update{
				Update: &fspb.Document{
					Name:   docPath,
					Fields: writeFields,
				},
			},
			CurrentDocument: precond,
		}
		if mask != nil {
			w.UpdateMask = &fspb.DocumentMask{FieldPaths: mask}
		}
		writes = append(writes, w)
		precond = nil // don't need precond in transform if it is in write
	}
	if transform != nil {
		var fts []*fspb.DocumentTransform_FieldTransform
		for _, p := range transform {
			fts = append(fts, &fspb.DocumentTransform_FieldTransform{
				FieldPath: p,
				TransformType: &fspb.DocumentTransform_FieldTransform_SetToServerValue{
					fspb.DocumentTransform_FieldTransform_REQUEST_TIME,
				},
			})
		}
		writes = append(writes, &fspb.Write{
			Operation: &fspb.Write_Transform{
				&fspb.DocumentTransform{
					Document:        docPath,
					FieldTransforms: fts,
				},
			},
			CurrentDocument: precond,
		})
	}
	return &fspb.CommitRequest{
		Database: database,
		Writes:   writes,
	}
}

var mergeAllOption = &tpb.SetOption{All: true}

func mergeOption(paths ...[]string) *tpb.SetOption {
	return &tpb.SetOption{Fields: toFieldPaths(paths)}
}

// A queryTest describes a series of function calls to create a Query.
type queryTest struct {
	suffix  string                // textproto filename suffix
	desc    string                // short description
	comment string                // detailed explanation (comment in textproto file)
	clauses []interface{}         // the query clauses (corresponding to function calls)
	query   *fspb.StructuredQuery // the desired proto
	isErr   bool                  // arguments result in a client-side error
}

func genQuery(suite *tpb.TestSuite) {
	docsnap := &tpb.Cursor{
		DocSnapshot: &tpb.DocSnapshot{
			Path:     collPath + "/D",
			JsonData: `{"a": 7, "b": 8}`,
		},
	}
	badDocsnap := &tpb.Cursor{
		DocSnapshot: &tpb.DocSnapshot{
			Path:     database + "/documents/C2/D",
			JsonData: `{"a": 7, "b": 8}`,
		},
	}
	docsnapRef := refval(collPath + "/D")
	for _, test := range []queryTest{
		{
			suffix:  "select-empty",
			desc:    "empty Select clause",
			comment: `An empty Select clause selects just the document ID.`,
			clauses: []interface{}{&tpb.Select{[]*tpb.FieldPath{}}},
			query: &fspb.StructuredQuery{
				Select: &fspb.StructuredQuery_Projection{
					Fields: []*fspb.StructuredQuery_FieldReference{{"__name__"}},
				},
			},
		},
		{
			suffix:  "select",
			desc:    "Select clause with some fields",
			comment: `An ordinary Select clause.`,
			clauses: []interface{}{
				&tpb.Select{[]*tpb.FieldPath{fp("a"), fp("b")}},
			},
			query: &fspb.StructuredQuery{
				Select: &fspb.StructuredQuery_Projection{
					Fields: []*fspb.StructuredQuery_FieldReference{{"a"}, {"b"}},
				},
			},
		},
		{
			suffix:  "select-last-wins",
			desc:    "two Select clauses",
			comment: `The last Select clause is the only one used.`,
			clauses: []interface{}{
				&tpb.Select{[]*tpb.FieldPath{fp("a"), fp("b")}},
				&tpb.Select{[]*tpb.FieldPath{fp("c")}},
			},
			query: &fspb.StructuredQuery{
				Select: &fspb.StructuredQuery_Projection{
					Fields: []*fspb.StructuredQuery_FieldReference{{"c"}},
				},
			},
		},
		{
			suffix:  "where",
			desc:    "Where clause",
			comment: `A simple Where clause.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: ">", JsonValue: `5`},
			},
			query: &fspb.StructuredQuery{
				Where: filter("a", fspb.StructuredQuery_FieldFilter_GREATER_THAN, 5),
			},
		},
		{
			suffix:  "where-2",
			desc:    "two Where clauses",
			comment: `Multiple Where clauses are combined into a composite filter.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: ">=", JsonValue: `5`},
				&tpb.Where{Path: fp("b"), Op: "<", JsonValue: `"foo"`},
			},
			query: &fspb.StructuredQuery{
				Where: &fspb.StructuredQuery_Filter{
					&fspb.StructuredQuery_Filter_CompositeFilter{
						&fspb.StructuredQuery_CompositeFilter{
							Op: fspb.StructuredQuery_CompositeFilter_AND,
							Filters: []*fspb.StructuredQuery_Filter{
								filter("a", fspb.StructuredQuery_FieldFilter_GREATER_THAN_OR_EQUAL, 5),
								filter("b", fspb.StructuredQuery_FieldFilter_LESS_THAN, "foo"),
							},
						},
					},
				},
			},
		},

		{
			suffix:  "where-null",
			desc:    "a Where clause comparing to null",
			comment: "A Where clause that tests for equality with null results in a unary filter.",
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "==", JsonValue: `null`},
			},
			query: &fspb.StructuredQuery{
				Where: unaryFilter("a", fspb.StructuredQuery_UnaryFilter_IS_NULL),
			},
		},
		{
			suffix:  "where-NaN",
			desc:    "a Where clause comparing to NaN",
			comment: "A Where clause that tests for equality with NaN results in a unary filter.",
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "==", JsonValue: `"NaN"`},
			},
			query: &fspb.StructuredQuery{
				Where: unaryFilter("a", fspb.StructuredQuery_UnaryFilter_IS_NAN),
			},
		},
		{
			suffix:  "offset-limit",
			desc:    "Offset and Limit clauses",
			comment: `Offset and Limit clauses.`,
			clauses: []interface{}{&tpb.Clause_Offset{2}, &tpb.Clause_Limit{3}},
			query: &fspb.StructuredQuery{
				Offset: 2,
				Limit:  &wrappers.Int32Value{3},
			},
		},
		{
			suffix:  "offset-limit-last-wins",
			desc:    "multiple Offset and Limit clauses",
			comment: `With multiple Offset or Limit clauses, the last one wins.`,
			clauses: []interface{}{
				&tpb.Clause_Offset{2},
				&tpb.Clause_Limit{3},
				&tpb.Clause_Limit{4},
				&tpb.Clause_Offset{5},
			},
			query: &fspb.StructuredQuery{
				Offset: 5,
				Limit:  &wrappers.Int32Value{4},
			},
		},
		{
			suffix:  "order",
			desc:    "basic OrderBy clauses",
			comment: `Multiple OrderBy clauses combine.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("b"), "asc"},
				&tpb.OrderBy{fp("a"), "desc"},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("b"), fspb.StructuredQuery_ASCENDING},
					{fref("a"), fspb.StructuredQuery_DESCENDING},
				},
			},
		},
		{
			suffix:  "cursor-vals-1a",
			desc:    "StartAt/EndBefore with values",
			comment: `Cursor methods take the same number of values as there are OrderBy clauses.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "asc"},
				&tpb.Clause_StartAt{&tpb.Cursor{JsonValues: []string{`7`}}},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`9`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("a"), fspb.StructuredQuery_ASCENDING},
				},
				StartAt: &fspb.Cursor{Values: []*fspb.Value{val(7)}, Before: true},
				EndAt:   &fspb.Cursor{Values: []*fspb.Value{val(9)}, Before: true},
			},
		},
		{
			suffix:  "cursor-vals-1b",
			desc:    "StartAfter/EndAt with values",
			comment: `Cursor methods take the same number of values as there are OrderBy clauses.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "asc"},
				&tpb.Clause_StartAfter{&tpb.Cursor{JsonValues: []string{`7`}}},
				&tpb.Clause_EndAt{&tpb.Cursor{JsonValues: []string{`9`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("a"), fspb.StructuredQuery_ASCENDING},
				},
				StartAt: &fspb.Cursor{Values: []*fspb.Value{val(7)}, Before: false},
				EndAt:   &fspb.Cursor{Values: []*fspb.Value{val(9)}, Before: false},
			},
		},
		{
			suffix:  "cursor-vals-2",
			desc:    "Start/End with two values",
			comment: `Cursor methods take the same number of values as there are OrderBy clauses.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "asc"},
				&tpb.OrderBy{fp("b"), "desc"},
				&tpb.Clause_StartAt{&tpb.Cursor{JsonValues: []string{`7`, `8`}}},
				&tpb.Clause_EndAt{&tpb.Cursor{JsonValues: []string{`9`, `10`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("a"), fspb.StructuredQuery_ASCENDING},
					{fref("b"), fspb.StructuredQuery_DESCENDING},
				},
				StartAt: &fspb.Cursor{Values: []*fspb.Value{val(7), val(8)}, Before: true},
				EndAt:   &fspb.Cursor{Values: []*fspb.Value{val(9), val(10)}, Before: false},
			},
		},
		{
			suffix: "cursor-vals-docid",
			desc:   "cursor methods with __name__",
			comment: `Cursor values corresponding to a __name__ field take the document path relative to the
query's collection.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("__name__"), "asc"},
				&tpb.Clause_StartAfter{&tpb.Cursor{JsonValues: []string{`"D1"`}}},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`"D2"`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("__name__"), fspb.StructuredQuery_ASCENDING},
				},
				StartAt: &fspb.Cursor{
					Values: []*fspb.Value{refval(collPath + "/D1")},
					Before: false,
				},
				EndAt: &fspb.Cursor{
					Values: []*fspb.Value{refval(collPath + "/D2")},
					Before: true,
				},
			},
		},
		{
			suffix:  "cursor-vals-last-wins",
			desc:    "cursor methods, last one wins",
			comment: `When multiple Start* or End* calls occur, the values of the last one are used.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "asc"},
				&tpb.Clause_StartAfter{&tpb.Cursor{JsonValues: []string{`1`}}},
				&tpb.Clause_StartAt{&tpb.Cursor{JsonValues: []string{`2`}}},
				&tpb.Clause_EndAt{&tpb.Cursor{JsonValues: []string{`3`}}},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`4`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("a"), fspb.StructuredQuery_ASCENDING},
				},
				StartAt: &fspb.Cursor{
					Values: []*fspb.Value{val(2)},
					Before: true,
				},
				EndAt: &fspb.Cursor{
					Values: []*fspb.Value{val(4)},
					Before: true,
				},
			},
		},
		{
			suffix:  "cursor-docsnap",
			desc:    "cursor methods with a document snapshot",
			comment: `When a document snapshot is used, the client appends a __name__ order-by clause.`,
			clauses: []interface{}{
				&tpb.Clause_StartAt{docsnap},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("__name__"), fspb.StructuredQuery_ASCENDING},
				},
				StartAt: &fspb.Cursor{
					Values: []*fspb.Value{docsnapRef},
					Before: true,
				},
			},
		},
		{
			suffix: "cursor-docsnap-order",
			desc:   "cursor methods with a document snapshot, existing orderBy",
			comment: `When a document snapshot is used, the client appends a __name__ order-by clause
with the direction of the last order-by clause.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "asc"},
				&tpb.OrderBy{fp("b"), "desc"},
				&tpb.Clause_StartAfter{docsnap},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("a"), fspb.StructuredQuery_ASCENDING},
					{fref("b"), fspb.StructuredQuery_DESCENDING},
					{fref("__name__"), fspb.StructuredQuery_DESCENDING},
				},
				StartAt: &fspb.Cursor{
					Values: []*fspb.Value{val(7), val(8), docsnapRef},
					Before: false,
				},
			},
		},
		{
			suffix:  "cursor-docsnap-where-eq",
			desc:    "cursor methods with a document snapshot and an equality where clause",
			comment: `A Where clause using equality doesn't change the implicit orderBy clauses.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "==", JsonValue: `3`},
				&tpb.Clause_EndAt{docsnap},
			},
			query: &fspb.StructuredQuery{
				Where: filter("a", fspb.StructuredQuery_FieldFilter_EQUAL, 3),
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("__name__"), fspb.StructuredQuery_ASCENDING},
				},
				EndAt: &fspb.Cursor{
					Values: []*fspb.Value{docsnapRef},
					Before: false,
				},
			},
		},
		{
			suffix: "cursor-docsnap-where-neq",
			desc:   "cursor method with a document snapshot and an inequality where clause",
			comment: `A Where clause with an inequality results in an OrderBy clause
on that clause's path, if there are no other OrderBy clauses.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "<=", JsonValue: `3`},
				&tpb.Clause_EndBefore{docsnap},
			},
			query: &fspb.StructuredQuery{
				Where: filter("a", fspb.StructuredQuery_FieldFilter_LESS_THAN_OR_EQUAL, 3),
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("a"), fspb.StructuredQuery_ASCENDING},
					{fref("__name__"), fspb.StructuredQuery_ASCENDING},
				},
				EndAt: &fspb.Cursor{
					Values: []*fspb.Value{val(7), docsnapRef},
					Before: true,
				},
			},
		},
		{
			suffix: "cursor-docsnap-where-neq-orderby",
			desc:   "cursor method, doc snapshot, inequality where clause, and existing orderBy clause",
			comment: `If there is an OrderBy clause, the inequality Where clause does
not result in a new OrderBy clause. We still add a __name__ OrderBy clause`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "desc"},
				&tpb.Where{Path: fp("a"), Op: "<", JsonValue: `4`},
				&tpb.Clause_StartAt{docsnap},
			},
			query: &fspb.StructuredQuery{
				Where: filter("a", fspb.StructuredQuery_FieldFilter_LESS_THAN, 4),
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("a"), fspb.StructuredQuery_DESCENDING},
					{fref("__name__"), fspb.StructuredQuery_DESCENDING},
				},
				StartAt: &fspb.Cursor{
					Values: []*fspb.Value{val(7), docsnapRef},
					Before: true,
				},
			},
		},
		{
			suffix: "cursor-docsnap-orderby-name",
			desc:   "cursor method, doc snapshot, existing orderBy __name__",
			comment: `If there is an existing orderBy clause on __name__,
no changes are made to the list of orderBy clauses.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "desc"},
				&tpb.OrderBy{fp("__name__"), "asc"},
				&tpb.Clause_StartAt{docsnap},
				&tpb.Clause_EndAt{docsnap},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{fref("a"), fspb.StructuredQuery_DESCENDING},
					{fref("__name__"), fspb.StructuredQuery_ASCENDING},
				},
				StartAt: &fspb.Cursor{
					Values: []*fspb.Value{val(7), docsnapRef},
					Before: true,
				},
				EndAt: &fspb.Cursor{
					Values: []*fspb.Value{val(7), docsnapRef},
					Before: false,
				},
			},
		},
		// Errors
		{
			suffix:  "invalid-operator",
			desc:    "invalid operator in Where clause",
			comment: "The !=  operator is not supported.",
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "!=", JsonValue: `4`},
			},
			isErr: true,
		},
		{
			suffix:  "invalid-path-select",
			desc:    "invalid path in Where clause",
			comment: "The path has an empty component.",
			clauses: []interface{}{
				&tpb.Select{[]*tpb.FieldPath{{[]string{"*", ""}}}},
			},
			isErr: true,
		},
		{
			suffix:  "invalid-path-where",
			desc:    "invalid path in Where clause",
			comment: "The path has an empty component.",
			clauses: []interface{}{
				&tpb.Where{Path: &tpb.FieldPath{[]string{"*", ""}}, Op: "==", JsonValue: `4`},
			},
			isErr: true,
		},
		{
			suffix:  "invalid-path-order",
			desc:    "invalid path in OrderBy clause",
			comment: "The path has an empty component.",
			clauses: []interface{}{
				&tpb.OrderBy{&tpb.FieldPath{[]string{"*", ""}}, "asc"},
			},
			isErr: true,
		},
		{
			suffix: "cursor-no-order",
			desc:   "cursor method without orderBy",
			comment: `If a cursor method with a list of values is provided, there must be at least as many
explicit orderBy clauses as values.`,
			clauses: []interface{}{
				&tpb.Clause_StartAt{&tpb.Cursor{JsonValues: []string{`2`}}},
			},
			isErr: true,
		},
		{
			suffix:  "st-where",
			desc:    "ServerTimestamp in Where",
			comment: `Sentinel values are not permitted in queries.`,
			clauses: []interface{}{
				&tpb.Where{fp("a"), "==", `"ServerTimestamp"`},
			},
			isErr: true,
		},
		{
			suffix:  "del-where",
			desc:    "Delete in Where",
			comment: `Sentinel values are not permitted in queries.`,
			clauses: []interface{}{
				&tpb.Where{fp("a"), "==", `"Delete"`},
			},
			isErr: true,
		},
		{
			suffix:  "st-cursor",
			desc:    "ServerTimestamp in cursor method",
			comment: `Sentinel values are not permitted in queries.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "asc"},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`"ServerTimestamp"`}}},
			},
			isErr: true,
		},
		{
			suffix:  "del-cursor",
			desc:    "Delete in cursor method",
			comment: `Sentinel values are not permitted in queries.`,
			clauses: []interface{}{
				&tpb.OrderBy{fp("a"), "asc"},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`"Delete"`}}},
			},
			isErr: true,
		},
		{
			suffix: "wrong-collection",
			desc:   "doc snapshot with wrong collection in cursor method",
			comment: `If a document snapshot is passed to a Start*/End* method, it must be in the
same collection as the query.`,
			clauses: []interface{}{
				&tpb.Clause_EndBefore{badDocsnap},
			},
			isErr: true,
		},
		{
			suffix:  "bad-null",
			desc:    "where clause with non-== comparison with Null",
			comment: `You can only compare Null for equality.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: ">", JsonValue: `null`},
			},
			isErr: true,
		},
		{
			suffix:  "bad-NaN",
			desc:    "where clause with non-== comparison with NaN",
			comment: `You can only compare NaN for equality.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "<", JsonValue: `"NaN"`},
			},
			isErr: true,
		},
	} {
		var tclauses []*tpb.Clause
		for _, c := range test.clauses {
			tclauses = append(tclauses, toClause(c))
		}
		query := test.query
		if query != nil {
			query.From = []*fspb.StructuredQuery_CollectionSelector{{CollectionId: "C"}}
		}
		tp := &tpb.Test{
			Description: "query: " + test.desc,
			Test: &tpb.Test_Query{&tpb.QueryTest{
				CollPath: collPath,
				Clauses:  tclauses,
				Query:    query,
				IsError:  test.isErr,
			}},
		}
		suite.Tests = append(suite.Tests, tp)
		outputTestText(fmt.Sprintf("query-%s", test.suffix), test.comment, tp)
	}
}

func toClause(m interface{}) *tpb.Clause {
	switch c := m.(type) {
	case *tpb.Select:
		return &tpb.Clause{&tpb.Clause_Select{c}}
	case *tpb.Where:
		return &tpb.Clause{&tpb.Clause_Where{c}}
	case *tpb.OrderBy:
		return &tpb.Clause{&tpb.Clause_OrderBy{c}}
	case *tpb.Clause_Offset:
		return &tpb.Clause{c}
	case *tpb.Clause_Limit:
		return &tpb.Clause{c}
	case *tpb.Clause_StartAt:
		return &tpb.Clause{c}
	case *tpb.Clause_StartAfter:
		return &tpb.Clause{c}
	case *tpb.Clause_EndAt:
		return &tpb.Clause{c}
	case *tpb.Clause_EndBefore:
		return &tpb.Clause{c}
	default:
		panic("unknown clause type")
	}
}

func toFieldPaths(fps [][]string) []*tpb.FieldPath {
	var ps []*tpb.FieldPath
	for _, fp := range fps {
		ps = append(ps, &tpb.FieldPath{fp})
	}
	return ps
}

func filter(field string, op fspb.StructuredQuery_FieldFilter_Operator, v interface{}) *fspb.StructuredQuery_Filter {
	return &fspb.StructuredQuery_Filter{
		&fspb.StructuredQuery_Filter_FieldFilter{
			&fspb.StructuredQuery_FieldFilter{
				Field: fref(field),
				Op:    op,
				Value: val(v),
			},
		},
	}
}

func unaryFilter(field string, op fspb.StructuredQuery_UnaryFilter_Operator) *fspb.StructuredQuery_Filter {
	return &fspb.StructuredQuery_Filter{
		FilterType: &fspb.StructuredQuery_Filter_UnaryFilter{
			UnaryFilter: &fspb.StructuredQuery_UnaryFilter{
				OperandType: &fspb.StructuredQuery_UnaryFilter_Field{
					Field: fref(field),
				},
				Op: op,
			},
		},
	}
}

var filenames = map[string]bool{}

func outputTestText(filename, comment string, t *tpb.Test) {
	if strings.HasSuffix(filename, "-") {
		log.Fatalf("test %q missing suffix", t.Description)
	}
	if strings.ContainsAny(filename, " \t\n',") {
		log.Fatalf("bad character in filename %q", filename)
	}
	if filenames[filename] {
		log.Fatalf("duplicate filename %q", filename)
	}
	filenames[filename] = true
	basename := filepath.Join(*outputDir, filename+".textproto")
	if err := writeTestToFile(basename, comment, t); err != nil {
		log.Fatalf("writing test: %v", err)
	}
	nTests++
}

func writeTestToFile(pathname, comment string, t *tpb.Test) (err error) {
	f, err := os.Create(pathname)
	if err != nil {
		return err
	}
	defer func() {
		err2 := f.Close()
		if err == nil {
			err = err2
		}
	}()

	fmt.Fprintln(f, "# DO NOT MODIFY. This file was generated by")
	fmt.Fprintln(f, "# github.com/GoogleCloudPlatform/google-cloud-common/testing/firestore/cmd/generate-firestore-tests/generate-firestore-tests.go.")
	fmt.Fprintln(f)
	doc.ToText(f, comment, "# ", "#    ", 80)
	fmt.Fprintln(f)
	return proto.MarshalText(f, t)
}

func writeProtoToFile(filename string, p proto.Message) (err error) {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		err2 := f.Close()
		if err == nil {
			err = err2
		}
	}()
	bytes, err := proto.Marshal(p)
	if err != nil {
		return err
	}
	_, err = f.Write(bytes)
	return err
}

func mp(args ...interface{}) map[string]*fspb.Value {
	if len(args)%2 != 0 {
		log.Fatalf("got %d args, want even number", len(args))
	}
	m := map[string]*fspb.Value{}
	for i := 0; i < len(args); i += 2 {
		m[args[i].(string)] = val(args[i+1])
	}
	return m
}

func val(a interface{}) *fspb.Value {
	switch x := a.(type) {
	case int:
		return &fspb.Value{&fspb.Value_IntegerValue{int64(x)}}
	case float64:
		return &fspb.Value{&fspb.Value_DoubleValue{x}}
	case bool:
		return &fspb.Value{&fspb.Value_BooleanValue{x}}
	case string:
		return &fspb.Value{&fspb.Value_StringValue{x}}
	case map[string]*fspb.Value:
		return &fspb.Value{&fspb.Value_MapValue{&fspb.MapValue{x}}}
	case []interface{}:
		var vals []*fspb.Value
		for _, e := range x {
			vals = append(vals, val(e))
		}
		return &fspb.Value{&fspb.Value_ArrayValue{&fspb.ArrayValue{vals}}}
	default:
		log.Fatalf("val: bad type: %T", a)
		return nil
	}
}

func refval(path string) *fspb.Value {
	return &fspb.Value{&fspb.Value_ReferenceValue{path}}
}

func fp(s string) *tpb.FieldPath {
	return &tpb.FieldPath{[]string{s}}
}

func fref(s string) *fspb.StructuredQuery_FieldReference {
	return &fspb.StructuredQuery_FieldReference{s}
}
