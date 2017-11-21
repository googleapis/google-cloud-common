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
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	tpb "cloud.google.com/go/firestore/genproto"
	"github.com/golang/protobuf/proto"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	fspb "google.golang.org/genproto/googleapis/firestore/v1beta1"
)

const (
	database = "projects/projectID/databases/(default)"
	docPath  = database + "/documents/C/d"
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
	binf, err := os.Create(filepath.Join(*outputDir, "tests.binprotos"))
	if err != nil {
		log.Fatal(err)
	}
	genGet(binf)
	genCreate(binf)
	genSet(binf)
	genUpdate(binf)
	genUpdatePaths(binf)
	genDelete(binf)
	if err := binf.Close(); err != nil {
		log.Fatalf("closing binary file: %v", err)
	}
	fmt.Printf("wrote %d tests to %s\n", nTests, *outputDir)
}

func genGet(binw io.Writer) {
	outputTest("get-basic", "A call to DocumentRef.Get.", binw, &tpb.Test{
		Description: "get: get a document",
		Test: &tpb.Test_Get{&tpb.GetTest{
			DocRefPath: docPath,
			Request:    &fspb.GetDocumentRequest{Name: docPath},
		}},
	})
}

func genCreate(binw io.Writer) {
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
		outputTest(fmt.Sprintf("create-%s", test.suffix), test.comment, binw, tp)
	}

}
func genSet(binw io.Writer) {
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
			inData:  `{"h": {"g": 5, "f": 6}, "e": 7}`,
			opt:     mergeOption([]string{"h"}),
			outData: mp("h", mp("g", 5, "f", 6)),
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
			suffix: "merge-nowrite",
			desc:   "If no ordinary values in Merge, no write",
			comment: `If all the fields in the merge option have ServerTimestamp
values, then no update operation is produced, only a transform.`,
			inData:    `{"a": 1, "b": "ServerTimestamp"}`,
			opt:       mergeOption([]string{"b"}),
			transform: []string{"b"},
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
			suffix: "mergeall-empty",
			desc:   "MergeAll cannot be specified with empty data.",
			comment: `It makes no sense to specify MergeAll and provide no data, so we
disallow it on the client.`,
			inData: `{}`,
			opt:    mergeAllOption,
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

		outputTest(fmt.Sprintf("set-%s", test.suffix), test.comment, binw, tp)
	}
}

func genUpdate(binw io.Writer) {
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
		outputTest(fmt.Sprintf("update-%s", test.suffix), comment, binw, tp)
	}
}

func genUpdatePaths(binw io.Writer) {
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
		outputTest(fmt.Sprintf("update-paths-%s", test.suffix), test.comment, binw, tp)
	}
}

func genDelete(binw io.Writer) {
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
		outputTest(fmt.Sprintf("delete-%s", test.suffix), test.comment, binw, tp)
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

func toFieldPaths(fps [][]string) []*tpb.FieldPath {
	var ps []*tpb.FieldPath
	for _, fp := range fps {
		ps = append(ps, &tpb.FieldPath{fp})
	}
	return ps
}

var filenames = map[string]bool{}

func outputTest(filename, comment string, binw io.Writer, t *tpb.Test) {
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
	if err := writeTestToFile(basename, comment, binw, t); err != nil {
		log.Fatalf("writing test: %v", err)
	}
	nTests++
}

func writeTestToFile(pathname, comment string, binw io.Writer, t *tpb.Test) (err error) {
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
	if err := proto.MarshalText(f, t); err != nil {
		return err
	}

	// Write binary protos to a single file, each preceded by its length as a varint.
	bytes, err := proto.Marshal(t)
	if err != nil {
		return err
	}
	if _, err = binw.Write(proto.EncodeVarint(uint64(len(bytes)))); err != nil {
		return err
	}
	_, err = binw.Write(bytes)
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
