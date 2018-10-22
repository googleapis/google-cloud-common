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
	database      = "projects/projectID/databases/(default)"
	collPath      = database + "/documents/C"
	docPath       = collPath + "/d"
	watchTargetID = 1
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

	outData       map[string]*fspb.Value                   // expected data in update write
	mask          []string                                 // expected fields in update mask
	maskForUpdate []string                                 // mask, but only for Update/UpdatePaths
	transform     []*fspb.DocumentTransform_FieldTransform // expected transformations
	isErr         bool                                     // arguments result in a client-side error
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
			transform:     transforms(st("a")),
		},
		{
			suffix: "arrayunion-alone",
			desc:   "ArrayUnion alone",
			comment: `If the only values in the input are ArrayUnion, then no
update operation should be produced.`,
			inData:        `{"a": ["ArrayUnion", 1, 2, 3]}`,
			paths:         [][]string{{"a"}},
			values:        []string{`["ArrayUnion", 1, 2, 3]`},
			outData:       nil,
			maskForUpdate: nil,
			transform:     transforms(arrayUnion("a", 1, 2, 3)),
		},
		{
			suffix: "arrayremove-alone",
			desc:   "ArrayRemove alone",
			comment: `If the only values in the input are ArrayRemove, then no
update operation should be produced.`,
			inData:        `{"a": ["ArrayRemove", 1, 2, 3]}`,
			paths:         [][]string{{"a"}},
			values:        []string{`["ArrayRemove", 1, 2, 3]`},
			outData:       nil,
			maskForUpdate: nil,
			transform:     transforms(arrayRemove("a", 1, 2, 3)),
		},
	}

	transformTests = []writeTest{
		{
			suffix:        "all-transforms",
			desc:          "all transforms in a single call",
			comment:       `A document can be created with any amount of transforms.`,
			inData:        `{"a": 1, "b": "ServerTimestamp", "c": ["ArrayUnion", 1, 2, 3], "d": ["ArrayRemove", 4, 5, 6]}`,
			paths:         [][]string{{"a"}, {"b"}, {"c"}, {"d"}},
			values:        []string{`1`, `"ServerTimestamp"`, `["ArrayUnion", 1, 2, 3]`, `["ArrayRemove", 4, 5, 6]`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a"},
			transform:     transforms(st("b"), arrayUnion("c", 1, 2, 3), arrayRemove("d", 4, 5, 6)),
		},
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
			transform:     transforms(st("b")),
		},
		{
			suffix: "arrayunion",
			desc:   "ArrayUnion with data",
			comment: `A key with ArrayUnion is removed from the data in the update 
operation. Instead it appears in a separate Transform operation.`,
			inData:        `{"a": 1, "b": ["ArrayUnion", 1, 2, 3]}`,
			paths:         [][]string{{"a"}, {"b"}},
			values:        []string{`1`, `["ArrayUnion", 1, 2, 3]`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a"},
			transform:     transforms(arrayUnion("b", 1, 2, 3)),
		},
		{
			suffix: "arrayremove",
			desc:   "ArrayRemove with data",
			comment: `A key with ArrayRemove is removed from the data in the update 
operation. Instead it appears in a separate Transform operation.`,
			inData:        `{"a": 1, "b": ["ArrayRemove", 1, 2, 3]}`,
			paths:         [][]string{{"a"}, {"b"}},
			values:        []string{`1`, `["ArrayRemove", 1, 2, 3]`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a"},
			transform:     transforms(arrayRemove("b", 1, 2, 3)),
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
			transform:     transforms(st("b.c")),
		},
		{
			suffix: "arrayunion-nested",
			desc:   "nested ArrayUnion field",
			comment: `An ArrayUnion value can occur at any depth. In this case,
the transform applies to the field path "b.c". Since "c" is removed from the update,
"b" becomes empty, so it is also removed from the update.`,
			inData:        `{"a": 1, "b": {"c": ["ArrayUnion", 1, 2, 3]}}`,
			paths:         [][]string{{"a"}, {"b"}},
			values:        []string{`1`, `{"c": ["ArrayUnion", 1, 2, 3]}`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a", "b"},
			transform:     transforms(arrayUnion("b.c", 1, 2, 3)),
		},
		{
			suffix: "arrayremove-nested",
			desc:   "nested ArrayRemove field",
			comment: `An ArrayRemove value can occur at any depth. In this case,
the transform applies to the field path "b.c". Since "c" is removed from the update,
"b" becomes empty, so it is also removed from the update.`,
			inData:        `{"a": 1, "b": {"c": ["ArrayRemove", 1, 2, 3]}}`,
			paths:         [][]string{{"a"}, {"b"}},
			values:        []string{`1`, `{"c": ["ArrayRemove", 1, 2, 3]}`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a", "b"},
			transform:     transforms(arrayRemove("b.c", 1, 2, 3)),
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
			transform:     transforms(st("b"), st("c.d")),
		},
		{
			suffix: "arrayunion-multi",
			desc:   "multiple ArrayUnion fields",
			comment: `A document can have more than one ArrayUnion field.
Since all the ArrayUnion fields are removed, the only field in the update is "a".`,
			commentForUpdate: `b is not in the mask because it will be set in the transform.
c must be in the mask: it should be replaced entirely. The transform will set c.d to the
timestamp, but the update will delete the rest of c.`,
			inData:        `{"a": 1, "b": ["ArrayUnion", 1, 2, 3], "c": {"d": ["ArrayUnion", 4, 5, 6]}}`,
			paths:         [][]string{{"a"}, {"b"}, {"c"}},
			values:        []string{`1`, `["ArrayUnion", 1, 2, 3]`, `{"d": ["ArrayUnion", 4, 5, 6]}`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a", "c"},
			transform:     transforms(arrayUnion("b", 1, 2, 3), arrayUnion("c.d", 4, 5, 6)),
		},
		{
			suffix: "arrayremove-multi",
			desc:   "multiple ArrayRemove fields",
			comment: `A document can have more than one ArrayRemove field.
Since all the ArrayRemove fields are removed, the only field in the update is "a".`,
			commentForUpdate: `b is not in the mask because it will be set in the transform.
c must be in the mask: it should be replaced entirely. The transform will set c.d to the
timestamp, but the update will delete the rest of c.`,
			inData:        `{"a": 1, "b": ["ArrayRemove", 1, 2, 3], "c": {"d": ["ArrayRemove", 4, 5, 6]}}`,
			paths:         [][]string{{"a"}, {"b"}, {"c"}},
			values:        []string{`1`, `["ArrayRemove", 1, 2, 3]`, `{"d": ["ArrayRemove", 4, 5, 6]}`},
			outData:       mp("a", 1),
			maskForUpdate: []string{"a", "c"},
			transform:     transforms(arrayRemove("b", 1, 2, 3), arrayRemove("c.d", 4, 5, 6)),
		},
		{
			suffix: "st-with-empty-map",
			desc:   "ServerTimestamp beside an empty map",
			comment: `When a ServerTimestamp and a map both reside inside a map, the
ServerTimestamp should be stripped out but the empty map should remain.`,
			inData:        `{"a": {"b": {}, "c": "ServerTimestamp"}}`,
			paths:         [][]string{{"a"}},
			values:        []string{`{"b": {}, "c": "ServerTimestamp"}`},
			outData:       mp("a", mp("b", mp())), // {"a": {"b": {}}}
			maskForUpdate: []string{"a"},
			transform:     transforms(st("a.c")),
		},
	}

	// Common errors with the transforms.
	transformErrorTests = []writeTest{
		{
			suffix: "arrayunion-with-st",
			desc:   "The ServerTimestamp sentinel cannot be in an ArrayUnion",
			comment: `The ServerTimestamp sentinel must be the value of a field. It may
not appear in an ArrayUnion.`,
			inData: `{"a": ["ArrayUnion", 1, "ServerTimestamp", 3]}`,
			paths:  [][]string{{"a"}},
			values: []string{`["ArrayUnion", 1, "ServerTimestamp", 3]`},
			isErr:  true,
		},
		{
			suffix: "arrayremove-with-st",
			desc:   "The ServerTimestamp sentinel cannot be in an ArrayUnion",
			comment: `The ServerTimestamp sentinel must be the value of a field. It may
not appear in an ArrayUnion.`,
			inData: `{"a": ["ArrayRemove", 1, "ServerTimestamp", 3]}`,
			paths:  [][]string{{"a"}},
			values: []string{`["ArrayRemove", 1, "ServerTimestamp", 3]`},
			isErr:  true,
		},
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
			suffix: "arrayunion-noarray",
			desc:   "ArrayUnion cannot be in an array value",
			comment: `ArrayUnion must be the value of a field. Firestore
transforms don't support array indexing.`,
			inData: `{"a": [1, 2, ["ArrayRemove", 1, 2, 3]]}`,
			paths:  [][]string{{"a"}},
			values: []string{`[1, 2, ["ArrayRemove", 1, 2, 3]]`},
			isErr:  true,
		},
		{
			suffix: "arrayremove-noarray",
			desc:   "ArrayRemove cannot be in an array value",
			comment: `ArrayRemove must be the value of a field. Firestore
transforms don't support array indexing.`,
			inData: `{"a": [1, 2, ["ArrayRemove", 1, 2, 3]]}`,
			paths:  [][]string{{"a"}},
			values: []string{`[1, 2, ["ArrayRemove", 1, 2, 3]]`},
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
		{
			suffix: "arrayunion-noarray-nested",
			desc:   "ArrayUnion cannot be anywhere inside an array value",
			comment: `There cannot be an array value anywhere on the path from the document
root to the ArrayUnion. Firestore transforms don't support array indexing.`,
			inData: `{"a": [1, {"b": ["ArrayUnion", 1, 2, 3]}]}`,
			paths:  [][]string{{"a"}},
			values: []string{`[1, {"b": ["ArrayUnion", 1, 2, 3]}]`},
			isErr:  true,
		},
		{
			suffix: "arrayremove-noarray-nested",
			desc:   "ArrayRemove cannot be anywhere inside an array value",
			comment: `There cannot be an array value anywhere on the path from the document
root to the ArrayRemove. Firestore transforms don't support array indexing.`,
			inData: `{"a": [1, {"b": ["ArrayRemove", 1, 2, 3]}]}`,
			paths:  [][]string{{"a"}},
			values: []string{`[1, {"b": ["ArrayRemove", 1, 2, 3]}]`},
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
	genListen(suite)
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
	tests = append(tests, transformTests...)
	tests = append(tests, transformErrorTests...)
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
		transform:     transforms(st("a")),
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
	tests = append(tests, transformTests...)
	tests = append(tests, transformErrorTests...)
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
			transform:     transforms(st("a")),
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
			transform: transforms(st("b")),
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
			transform:     transforms(st("a")),
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
			transform: transforms(st("b")),
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
			transform: transforms(st("b")),
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
			transform: transforms(st("h.g")),
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
			transform: transforms(st("h.g")),
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
			mask:    []string{},
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
	tests = append(tests, transformTests...)
	tests = append(tests, transformErrorTests...)
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
			transform: transforms(st("a.b.c")),
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
	tests = append(tests, transformTests...)
	tests = append(tests, transformErrorTests...)
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
		{
			suffix:  "fp-dup-transforms",
			desc:    "duplicate field path with only transforms",
			comment: `The same field cannot occur more than once, even if all the operations are transforms.`,
			paths:   [][]string{{"a"}, {"b"}, {"a"}},
			values:  []string{`["ArrayUnion", 1, 2, 3]`, `"ServerTimestamp"`, `["ArrayUnion", 4, 5, 6]`},
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

func newCommitRequest(writeFields map[string]*fspb.Value, mask []string, precond *fspb.Precondition, transforms []*fspb.DocumentTransform_FieldTransform) *fspb.CommitRequest {
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
	if transforms != nil {
		writes = append(writes, &fspb.Write{
			Operation: &fspb.Write_Transform{
				&fspb.DocumentTransform{
					Document:        docPath,
					FieldTransforms: transforms,
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
			clauses: []interface{}{&tpb.Select{Fields: []*tpb.FieldPath{}}},
			query: &fspb.StructuredQuery{
				Select: &fspb.StructuredQuery_Projection{
					Fields: []*fspb.StructuredQuery_FieldReference{fref("__name__")},
				},
			},
		},
		{
			suffix:  "select",
			desc:    "Select clause with some fields",
			comment: `An ordinary Select clause.`,
			clauses: []interface{}{
				&tpb.Select{Fields: []*tpb.FieldPath{fp("a"), fp("b")}},
			},
			query: &fspb.StructuredQuery{
				Select: &fspb.StructuredQuery_Projection{
					Fields: []*fspb.StructuredQuery_FieldReference{fref("a"), fref("b")},
				},
			},
		},
		{
			suffix:  "select-last-wins",
			desc:    "two Select clauses",
			comment: `The last Select clause is the only one used.`,
			clauses: []interface{}{
				&tpb.Select{Fields: []*tpb.FieldPath{fp("a"), fp("b")}},
				&tpb.Select{Fields: []*tpb.FieldPath{fp("c")}},
			},
			query: &fspb.StructuredQuery{
				Select: &fspb.StructuredQuery_Projection{
					Fields: []*fspb.StructuredQuery_FieldReference{fref("c")},
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
					FilterType: &fspb.StructuredQuery_Filter_CompositeFilter{
						CompositeFilter: &fspb.StructuredQuery_CompositeFilter{
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
				Limit:  &wrappers.Int32Value{Value: 3},
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
				Limit:  &wrappers.Int32Value{Value: 4},
			},
		},
		{
			suffix:  "order",
			desc:    "basic OrderBy clauses",
			comment: `Multiple OrderBy clauses combine.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("b"), Direction: "asc"},
				&tpb.OrderBy{Path: fp("a"), Direction: "desc"},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("b"), Direction: fspb.StructuredQuery_ASCENDING},
					{Field: fref("a"), Direction: fspb.StructuredQuery_DESCENDING},
				},
			},
		},
		{
			suffix:  "cursor-startat-empty-map",
			desc:    "StartAt with explicit empty map",
			comment: `Cursor methods are allowed to use empty maps with StartAt. It should result in an empty map in the query.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_StartAt{&tpb.Cursor{JsonValues: []string{`{}`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_ASCENDING},
				},
				StartAt: &fspb.Cursor{Values: []*fspb.Value{val(mp())}, Before: true},
			},
		},
		{
			suffix:  "cursor-endbefore-empty-map",
			desc:    "EndBefore with explicit empty map",
			comment: `Cursor methods are allowed to use empty maps with EndBefore. It should result in an empty map in the query.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`{}`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_ASCENDING},
				},
				EndAt: &fspb.Cursor{Values: []*fspb.Value{val(mp())}, Before: true},
			},
		},
		{
			suffix:  "cursor-startat-empty",
			desc:    "StartAt with empty values",
			comment: `Cursor methods are not allowed to use empty values with StartAt. It should result in an error.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_StartAt{&tpb.Cursor{}},
			},
			isErr: true,
		},
		{
			suffix:  "cursor-endbefore-empty",
			desc:    "EndBefore with empty values",
			comment: `Cursor methods are not allowed to use empty values with EndBefore. It should result in an error.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_EndBefore{&tpb.Cursor{}},
			},
			isErr: true,
		},
		{
			suffix:  "cursor-vals-1a",
			desc:    "StartAt/EndBefore with values",
			comment: `Cursor methods take the same number of values as there are OrderBy clauses.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_StartAt{&tpb.Cursor{JsonValues: []string{`7`}}},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`9`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_ASCENDING},
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
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_StartAfter{&tpb.Cursor{JsonValues: []string{`7`}}},
				&tpb.Clause_EndAt{&tpb.Cursor{JsonValues: []string{`9`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_ASCENDING},
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
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.OrderBy{Path: fp("b"), Direction: "desc"},
				&tpb.Clause_StartAt{&tpb.Cursor{JsonValues: []string{`7`, `8`}}},
				&tpb.Clause_EndAt{&tpb.Cursor{JsonValues: []string{`9`, `10`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_ASCENDING},
					{Field: fref("b"), Direction: fspb.StructuredQuery_DESCENDING},
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
				&tpb.OrderBy{Path: fp("__name__"), Direction: "asc"},
				&tpb.Clause_StartAfter{&tpb.Cursor{JsonValues: []string{`"D1"`}}},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`"D2"`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("__name__"), Direction: fspb.StructuredQuery_ASCENDING},
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
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_StartAfter{&tpb.Cursor{JsonValues: []string{`1`}}},
				&tpb.Clause_StartAt{&tpb.Cursor{JsonValues: []string{`2`}}},
				&tpb.Clause_EndAt{&tpb.Cursor{JsonValues: []string{`3`}}},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`4`}}},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_ASCENDING},
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
					{Field: fref("__name__"), Direction: fspb.StructuredQuery_ASCENDING},
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
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.OrderBy{Path: fp("b"), Direction: "desc"},
				&tpb.Clause_StartAfter{docsnap},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_ASCENDING},
					{Field: fref("b"), Direction: fspb.StructuredQuery_DESCENDING},
					{Field: fref("__name__"), Direction: fspb.StructuredQuery_DESCENDING},
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
					{Field: fref("__name__"), Direction: fspb.StructuredQuery_ASCENDING},
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
					{Field: fref("a"), Direction: fspb.StructuredQuery_ASCENDING},
					{Field: fref("__name__"), Direction: fspb.StructuredQuery_ASCENDING},
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
				&tpb.OrderBy{Path: fp("a"), Direction: "desc"},
				&tpb.Where{Path: fp("a"), Op: "<", JsonValue: `4`},
				&tpb.Clause_StartAt{docsnap},
			},
			query: &fspb.StructuredQuery{
				Where: filter("a", fspb.StructuredQuery_FieldFilter_LESS_THAN, 4),
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_DESCENDING},
					{Field: fref("__name__"), Direction: fspb.StructuredQuery_DESCENDING},
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
				&tpb.OrderBy{Path: fp("a"), Direction: "desc"},
				&tpb.OrderBy{Path: fp("__name__"), Direction: "asc"},
				&tpb.Clause_StartAt{docsnap},
				&tpb.Clause_EndAt{docsnap},
			},
			query: &fspb.StructuredQuery{
				OrderBy: []*fspb.StructuredQuery_Order{
					{Field: fref("a"), Direction: fspb.StructuredQuery_DESCENDING},
					{Field: fref("__name__"), Direction: fspb.StructuredQuery_ASCENDING},
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
				&tpb.Select{Fields: []*tpb.FieldPath{{Field: []string{"*", ""}}}},
			},
			isErr: true,
		},
		{
			suffix:  "invalid-path-where",
			desc:    "invalid path in Where clause",
			comment: "The path has an empty component.",
			clauses: []interface{}{
				&tpb.Where{Path: &tpb.FieldPath{Field: []string{"*", ""}}, Op: "==", JsonValue: `4`},
			},
			isErr: true,
		},
		{
			suffix:  "invalid-path-order",
			desc:    "invalid path in OrderBy clause",
			comment: "The path has an empty component.",
			clauses: []interface{}{
				&tpb.OrderBy{Path: &tpb.FieldPath{Field: []string{"*", ""}}, Direction: "asc"},
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
				&tpb.Where{Path: fp("a"), Op: "==", JsonValue: `"ServerTimestamp"`},
			},
			isErr: true,
		},
		{
			suffix:  "del-where",
			desc:    "Delete in Where",
			comment: `Sentinel values are not permitted in queries.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "==", JsonValue: `"Delete"`},
			},
			isErr: true,
		},
		{
			suffix:  "arrayunion-where",
			desc:    "ArrayUnion in Where",
			comment: `ArrayUnion is not permitted in queries.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "==", JsonValue: `["ArrayUnion", 1, 2, 3]`},
			},
			isErr: true,
		},
		{
			suffix:  "arrayremove-where",
			desc:    "ArrayRemove in Where",
			comment: `ArrayRemove is not permitted in queries.`,
			clauses: []interface{}{
				&tpb.Where{Path: fp("a"), Op: "==", JsonValue: `["ArrayRemove", 1, 2, 3]`},
			},
			isErr: true,
		},
		{
			suffix:  "st-cursor",
			desc:    "ServerTimestamp in cursor method",
			comment: `Sentinel values are not permitted in queries.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`"ServerTimestamp"`}}},
			},
			isErr: true,
		},
		{
			suffix:  "del-cursor",
			desc:    "Delete in cursor method",
			comment: `Sentinel values are not permitted in queries.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`"Delete"`}}},
			},
			isErr: true,
		},
		{
			suffix:  "arrayunion-cursor",
			desc:    "ArrayUnion in cursor method",
			comment: `ArrayUnion is not permitted in queries.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`["ArrayUnion", 1, 2, 3]`}}},
			},
			isErr: true,
		},
		{
			suffix:  "arrayremove-cursor",
			desc:    "ArrayRemove in cursor method",
			comment: `ArrayRemove is not permitted in queries.`,
			clauses: []interface{}{
				&tpb.OrderBy{Path: fp("a"), Direction: "asc"},
				&tpb.Clause_EndBefore{&tpb.Cursor{JsonValues: []string{`["ArrayRemove", 1, 2, 3]`}}},
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

// A listenTest describes a series of Listen RPC responses that result in one or more snapshots.
type listenTest struct {
	suffix    string                 // textproto filename suffix
	desc      string                 // short description
	comment   string                 // detailed explanation (comment in textproto file)
	responses []*fspb.ListenResponse // a sequence of responses sent over a Listen stream
	snapshots []*tpb.Snapshot
	isErr     bool // arguments result in a client-side error
}

func genListen(suite *tpb.TestSuite) {
	current := &fspb.ListenResponse{ResponseType: &fspb.ListenResponse_TargetChange{&fspb.TargetChange{
		TargetChangeType: fspb.TargetChange_CURRENT,
	}}}
	reset := &fspb.ListenResponse{ResponseType: &fspb.ListenResponse_TargetChange{&fspb.TargetChange{
		TargetChangeType: fspb.TargetChange_RESET,
	}}}

	noChange := func(readTime *tspb.Timestamp) *fspb.ListenResponse {
		return &fspb.ListenResponse{ResponseType: &fspb.ListenResponse_TargetChange{&fspb.TargetChange{
			TargetChangeType: fspb.TargetChange_NO_CHANGE,
			ReadTime:         readTime,
		}}}
	}

	change := func(doc *fspb.Document) *fspb.ListenResponse {
		return &fspb.ListenResponse{ResponseType: &fspb.ListenResponse_DocumentChange{&fspb.DocumentChange{
			Document:  doc,
			TargetIds: []int32{watchTargetID},
		}}}
	}

	del := func(path string) *fspb.ListenResponse {
		return &fspb.ListenResponse{ResponseType: &fspb.ListenResponse_DocumentDelete{&fspb.DocumentDelete{
			Document: collPath + "/" + path,
		}}}
	}

	filter := func(count int) *fspb.ListenResponse {
		return &fspb.ListenResponse{ResponseType: &fspb.ListenResponse_Filter{&fspb.ExistenceFilter{
			Count: int32(count),
		}}}
	}

	ts := func(secs int) *tspb.Timestamp {
		return &tspb.Timestamp{Seconds: int64(secs)}
	}

	doc := func(path string, aval int, utime *tspb.Timestamp) *fspb.Document {
		return &fspb.Document{
			Name:       collPath + "/" + path,
			Fields:     mp("a", aval),
			CreateTime: ts(1),
			UpdateTime: utime,
		}
	}

	added := func(doc *fspb.Document, idx int32) *tpb.DocChange {
		return &tpb.DocChange{
			Kind:     tpb.DocChange_ADDED,
			Doc:      doc,
			OldIndex: -1,
			NewIndex: idx,
		}
	}

	removed := func(doc *fspb.Document, idx int32) *tpb.DocChange {
		return &tpb.DocChange{
			Kind:     tpb.DocChange_REMOVED,
			Doc:      doc,
			OldIndex: idx,
			NewIndex: -1,
		}
	}

	modified := func(doc *fspb.Document, oidx, nidx int32) *tpb.DocChange {
		return &tpb.DocChange{
			Kind:     tpb.DocChange_MODIFIED,
			Doc:      doc,
			OldIndex: oidx,
			NewIndex: nidx,
		}
	}

	doc1 := doc("d1", 3, ts(1))
	doc1a := doc("d1", -1, ts(3))

	doc2 := doc("d2", 1, ts(1))
	doc3 := doc("d3", 1, ts(1))
	doc4 := doc("d4", 2, ts(1))
	doc4a := doc("d4", -2, ts(3))
	doc5 := doc("d5", 4, ts(1))
	doc6 := doc("d6", 3, ts(1))
	multiDocsTest := listenTest{
		suffix: "multi-docs",
		desc:   "multiple documents, added, deleted and updated",
		comment: `Changes should be ordered with deletes first, then additions, then mods,
each in query order.
Old indices refer to the immediately previous state, not the previous snapshot`,
		responses: []*fspb.ListenResponse{
			// First, add four docs.
			change(doc1), change(doc3), change(doc2), change(doc4),
			current, noChange(ts(2)),
			// Then: delete two, modify two, add two.
			change(doc5),
			del("d3"),
			change(doc1a),
			change(doc6),
			del("d2"),
			change(doc4a),
			noChange(ts(4)),
		},
		snapshots: []*tpb.Snapshot{
			{
				Docs: []*fspb.Document{doc2, doc3, doc4, doc1},
				Changes: []*tpb.DocChange{
					added(doc2, 0),
					added(doc3, 1),
					added(doc4, 2),
					added(doc1, 3),
				},
				ReadTime: ts(2),
			},
			{
				Docs: []*fspb.Document{doc4a, doc1a, doc6, doc5},
				Changes: []*tpb.DocChange{
					removed(doc2, 0),
					removed(doc3, 0),
					added(doc6, 2),
					added(doc5, 3),
					modified(doc4a, 0, 0),
					modified(doc1a, 1, 1),
				},
				ReadTime: ts(4),
			},
		},
	}

	doc1r := doc("d1", 2, ts(1))
	doc2r := doc("d2", 1, ts(2))
	doc2ra := doc("d2", 3, ts(3))
	doc3r := doc("d3", 3, ts(2))
	resetTest := listenTest{
		suffix: "reset",
		desc:   "RESET turns off CURRENT",
		comment: `A RESET message turns off the CURRENT state, and marks all documents as deleted.

If a document appeared on the stream but was never part of a snapshot ("d3" in this test), a reset
will make it disappear completely.

For a snapshot to happen at a NO_CHANGE reponse, we need to have both seen a CURRENT response, and
have a change from the previous snapshot. Here, after the reset, we see the same version of d2
again. That doesn't result in a snapshot.
`,
		responses: []*fspb.ListenResponse{
			change(doc1r),
			change(doc2r),
			current, noChange(ts(1)),
			change(doc3r),
			reset,
			noChange(ts(2)), // no snapshot because no longer current
			current,
			change(doc2ra),
			noChange(ts(3)),
			reset,
			change(doc2ra), // same docs as before, added back
			current,
			noChange(ts(4)), // no snapshot, because state is the same as the previous snapshot
			change(doc3r),
			noChange(ts(5)), // snapshot, because doc3r is new
		},
		snapshots: []*tpb.Snapshot{
			{
				Docs:     []*fspb.Document{doc2r, doc1r},
				Changes:  []*tpb.DocChange{added(doc2r, 0), added(doc1r, 1)},
				ReadTime: ts(1),
			},
			{
				Docs:     []*fspb.Document{doc2ra},
				Changes:  []*tpb.DocChange{removed(doc1r, 1), modified(doc2ra, 0, 0)},
				ReadTime: ts(3),
			},
			{
				Docs:     []*fspb.Document{doc2ra, doc3r},
				Changes:  []*tpb.DocChange{added(doc3r, 1)},
				ReadTime: ts(5),
			},
		},
	}

	for _, test := range []listenTest{
		{
			suffix:    "empty",
			desc:      "no changes; empty snapshot",
			comment:   `There are no changes, so the snapshot should be empty.`,
			responses: []*fspb.ListenResponse{current, noChange(ts(1))},
			snapshots: []*tpb.Snapshot{
				{
					ReadTime: ts(1),
				},
			},
		},
		{
			suffix:    "add-one",
			desc:      "add a doc",
			comment:   `Snapshot with a single document.`,
			responses: []*fspb.ListenResponse{change(doc("d1", 1, ts(1))), current, noChange(ts(2))},
			snapshots: []*tpb.Snapshot{
				{
					Docs:     []*fspb.Document{doc("d1", 1, ts(1))},
					Changes:  []*tpb.DocChange{added(doc("d1", 1, ts(1)), 0)},
					ReadTime: ts(2),
				},
			},
		},
		{
			suffix:  "add-mod-del-add",
			desc:    "add a doc, modify it, delete it, then add it again",
			comment: `Various changes to a single document.`,
			responses: []*fspb.ListenResponse{
				change(doc("d1", 1, ts(1))), current, noChange(ts(1)),
				change(doc("d1", 2, ts(2))), noChange(ts(2)), // different update time, so new snapshot
				del("d1"), noChange(ts(3)),
				change(doc("d1", 3, ts(3))), noChange(ts(4)),
			},
			snapshots: []*tpb.Snapshot{
				{
					Docs:     []*fspb.Document{doc("d1", 1, ts(1))},
					Changes:  []*tpb.DocChange{added(doc("d1", 1, ts(1)), 0)},
					ReadTime: ts(1),
				},
				{
					Docs:     []*fspb.Document{doc("d1", 2, ts(2))},
					Changes:  []*tpb.DocChange{modified(doc("d1", 2, ts(2)), 0, 0)},
					ReadTime: ts(2),
				},
				{
					Docs:     nil,
					Changes:  []*tpb.DocChange{removed(doc("d1", 2, ts(2)), 0)},
					ReadTime: ts(3),
				},
				{
					Docs:     []*fspb.Document{doc("d1", 3, ts(3))},
					Changes:  []*tpb.DocChange{added(doc("d1", 3, ts(3)), 0)},
					ReadTime: ts(4),
				},
			},
		},
		{
			suffix: "nomod",
			desc:   "add a doc, then change it but without changing its update time",
			comment: `Document updates are recognized by a change in the update time, not the data.
This shouldn't actually happen. It is just a test of the update logic.`,
			responses: []*fspb.ListenResponse{
				change(doc("d1", 1, ts(1))), current, noChange(ts(1)),
				change(doc("d1", 2, ts(1))), noChange(ts(2)), // same update time, so no snapshot
				del("d1"), noChange(ts(3)),
			},
			snapshots: []*tpb.Snapshot{
				{
					Docs:     []*fspb.Document{doc("d1", 1, ts(1))},
					Changes:  []*tpb.DocChange{added(doc("d1", 1, ts(1)), 0)},
					ReadTime: ts(1),
				},
				{
					Docs:     nil,
					Changes:  []*tpb.DocChange{removed(doc("d1", 1, ts(1)), 0)},
					ReadTime: ts(3),
				},
			},
		},
		{
			suffix: "add-three",
			desc:   "add three documents",
			comment: `A snapshot with three documents. The documents are sorted
first by the "a" field, then by their path. The changes are ordered the same way.`,
			responses: []*fspb.ListenResponse{
				change(doc("d1", 3, ts(1))),
				change(doc("d3", 1, ts(1))),
				change(doc("d2", 1, ts(1))),
				current, noChange(ts(2)),
			},
			snapshots: []*tpb.Snapshot{
				{
					Docs: []*fspb.Document{
						doc("d2", 1, ts(1)), // same value, so ordered by path
						doc("d3", 1, ts(1)),
						doc("d1", 3, ts(1)),
					},
					Changes: []*tpb.DocChange{
						added(doc("d2", 1, ts(1)), 0),
						added(doc("d3", 1, ts(1)), 1),
						added(doc("d1", 3, ts(1)), 2),
					},
					ReadTime: ts(2),
				},
			},
		},
		{
			suffix:  "nocurrent",
			desc:    "no snapshot if we don't see CURRENT",
			comment: `If the watch state is not marked CURRENT, no snapshot is issued.`,
			responses: []*fspb.ListenResponse{
				change(doc("d1", 1, ts(1))),
				noChange(ts(1)),
				change(doc("d2", 2, ts(2))),
				current, noChange(ts(2)),
			},
			snapshots: []*tpb.Snapshot{
				{
					Docs: []*fspb.Document{
						doc("d1", 1, ts(1)),
						doc("d2", 2, ts(2)),
					},
					Changes: []*tpb.DocChange{
						added(doc("d1", 1, ts(1)), 0),
						added(doc("d2", 2, ts(2)), 1),
					},
					ReadTime: ts(2),
				},
			},
		},
		multiDocsTest,
		resetTest,
		{
			suffix:  "doc-remove",
			desc:    "DocumentRemove behaves like DocumentDelete",
			comment: `The DocumentRemove response behaves exactly like DocumentDelete.`,
			responses: []*fspb.ListenResponse{
				change(doc1), current, noChange(ts(1)),
				&fspb.ListenResponse{ResponseType: &fspb.ListenResponse_DocumentRemove{&fspb.DocumentRemove{
					Document: doc1.Name,
				}}},
				noChange(ts(2)),
			},
			snapshots: []*tpb.Snapshot{
				{
					Docs:     []*fspb.Document{doc1},
					Changes:  []*tpb.DocChange{added(doc1, 0)},
					ReadTime: ts(1),
				},
				{
					Changes:  []*tpb.DocChange{removed(doc1, 0)},
					ReadTime: ts(2),
				},
			},
		},
		{
			suffix: "filter-nop",
			desc:   "Filter response with same size is a no-op",
			comment: `A Filter response whose count matches the size of the current
state (docs in last snapshot + docs added - docs deleted) is a no-op.`,
			responses: []*fspb.ListenResponse{
				change(doc1), change(doc2), current, noChange(ts(1)),
				change(doc3), del("d1"),
				filter(2),
				noChange(ts(2)),
			},
			snapshots: []*tpb.Snapshot{
				{
					Docs:     []*fspb.Document{doc2, doc1},
					Changes:  []*tpb.DocChange{added(doc2, 0), added(doc1, 1)},
					ReadTime: ts(1),
				},
				{
					Docs:     []*fspb.Document{doc2, doc3},
					Changes:  []*tpb.DocChange{removed(doc1, 1), added(doc3, 1)},
					ReadTime: ts(2),
				},
			},
		},
		{
			suffix: "removed-target-ids",
			desc:   "DocumentChange with removed_target_id is like a delete.",
			comment: `A DocumentChange with the watch target ID in the removed_target_ids field is the
same as deleting a document.`,
			responses: []*fspb.ListenResponse{
				change(doc1), current, noChange(ts(1)),
				&fspb.ListenResponse{ResponseType: &fspb.ListenResponse_DocumentChange{&fspb.DocumentChange{
					Document:         doc1,
					RemovedTargetIds: []int32{watchTargetID},
				}}},
				noChange(ts(2)),
			},
			snapshots: []*tpb.Snapshot{
				{
					Docs:     []*fspb.Document{doc1},
					Changes:  []*tpb.DocChange{added(doc1, 0)},
					ReadTime: ts(1),
				},
				{
					Docs:     nil,
					Changes:  []*tpb.DocChange{removed(doc1, 0)},
					ReadTime: ts(2),
				},
			},
		},
		{
			suffix:  "target-add-nop",
			desc:    "TargetChange_ADD is a no-op if it has the same target ID",
			comment: `A TargetChange_ADD response must have the same watch target ID.`,
			responses: []*fspb.ListenResponse{
				change(doc1), current,
				&fspb.ListenResponse{ResponseType: &fspb.ListenResponse_TargetChange{&fspb.TargetChange{
					TargetChangeType: fspb.TargetChange_ADD,
					TargetIds:        []int32{watchTargetID},
					ReadTime:         ts(2),
				}}},
				noChange(ts(1)),
			},
			snapshots: []*tpb.Snapshot{
				{
					Docs:     []*fspb.Document{doc1},
					Changes:  []*tpb.DocChange{added(doc1, 0)},
					ReadTime: ts(1),
				},
			},
		},
		// Errors
		{
			suffix:  "target-add-wrong-id",
			desc:    "TargetChange_ADD is an error if it has a different target ID",
			comment: `A TargetChange_ADD response must have the same watch target ID.`,
			responses: []*fspb.ListenResponse{
				change(doc1), current,
				&fspb.ListenResponse{ResponseType: &fspb.ListenResponse_TargetChange{&fspb.TargetChange{
					TargetChangeType: fspb.TargetChange_ADD,
					TargetIds:        []int32{watchTargetID + 1},
					ReadTime:         ts(2),
				}}},
				noChange(ts(1)),
			},
			isErr: true,
		},
		{
			suffix:  "target-remove",
			desc:    "TargetChange_REMOVE should not appear",
			comment: `A TargetChange_REMOVE response should never be sent.`,
			responses: []*fspb.ListenResponse{
				change(doc1), current,
				&fspb.ListenResponse{ResponseType: &fspb.ListenResponse_TargetChange{&fspb.TargetChange{
					TargetChangeType: fspb.TargetChange_REMOVE,
				}}},
				noChange(ts(1)),
			},
			isErr: true,
		},
	} {
		tp := &tpb.Test{
			Description: "listen: " + test.desc,
			Test: &tpb.Test_Listen{&tpb.ListenTest{
				Responses: test.responses,
				Snapshots: test.snapshots,
				IsError:   test.isErr,
			}},
		}
		suite.Tests = append(suite.Tests, tp)
		outputTestText(fmt.Sprintf("listen-%s", test.suffix), test.comment, tp)
	}
}

func toClause(m interface{}) *tpb.Clause {
	switch c := m.(type) {
	case *tpb.Select:
		return &tpb.Clause{Clause: &tpb.Clause_Select{c}}
	case *tpb.Where:
		return &tpb.Clause{Clause: &tpb.Clause_Where{c}}
	case *tpb.OrderBy:
		return &tpb.Clause{Clause: &tpb.Clause_OrderBy{c}}
	case *tpb.Clause_Offset:
		return &tpb.Clause{Clause: c}
	case *tpb.Clause_Limit:
		return &tpb.Clause{Clause: c}
	case *tpb.Clause_StartAt:
		return &tpb.Clause{Clause: c}
	case *tpb.Clause_StartAfter:
		return &tpb.Clause{Clause: c}
	case *tpb.Clause_EndAt:
		return &tpb.Clause{Clause: c}
	case *tpb.Clause_EndBefore:
		return &tpb.Clause{Clause: c}
	default:
		panic("unknown clause type")
	}
}

func toFieldPaths(fps [][]string) []*tpb.FieldPath {
	var ps []*tpb.FieldPath
	for _, fp := range fps {
		ps = append(ps, &tpb.FieldPath{Field: fp})
	}
	return ps
}

func filter(field string, op fspb.StructuredQuery_FieldFilter_Operator, v interface{}) *fspb.StructuredQuery_Filter {
	return &fspb.StructuredQuery_Filter{
		FilterType: &fspb.StructuredQuery_Filter_FieldFilter{
			FieldFilter: &fspb.StructuredQuery_FieldFilter{
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

// mp returns a map. Each odd element is a key, and the next (even) element is its
// value. If the total number of elements is odd, the last key's value will be nil.
// If there are no values, an empty map is returned.
func mp(args ...interface{}) map[string]*fspb.Value {
	m := map[string]*fspb.Value{}

	var k interface{}
	for i := 0; i < len(args); i++ {
		if i%2 == 0 {
			// key
			k = args[i]
			m[k.(string)] = nil
		} else {
			// val
			m[k.(string)] = val(args[i])
		}
	}
	return m
}

func val(a interface{}) *fspb.Value {
	switch x := a.(type) {
	case int:
		return &fspb.Value{ValueType: &fspb.Value_IntegerValue{int64(x)}}
	case float64:
		return &fspb.Value{ValueType: &fspb.Value_DoubleValue{x}}
	case bool:
		return &fspb.Value{ValueType: &fspb.Value_BooleanValue{x}}
	case string:
		return &fspb.Value{ValueType: &fspb.Value_StringValue{x}}
	case map[string]*fspb.Value:
		return &fspb.Value{ValueType: &fspb.Value_MapValue{&fspb.MapValue{Fields: x}}}
	case []interface{}:
		var vals []*fspb.Value
		for _, e := range x {
			vals = append(vals, val(e))
		}
		return &fspb.Value{ValueType: &fspb.Value_ArrayValue{&fspb.ArrayValue{Values: vals}}}
	default:
		log.Fatalf("val: bad type: %T", a)
		return nil
	}
}

func refval(path string) *fspb.Value {
	return &fspb.Value{ValueType: &fspb.Value_ReferenceValue{path}}
}

func fp(s string) *tpb.FieldPath {
	return &tpb.FieldPath{Field: []string{s}}
}

func fref(s string) *fspb.StructuredQuery_FieldReference {
	return &fspb.StructuredQuery_FieldReference{FieldPath: s}
}

func transforms(t ...*fspb.DocumentTransform_FieldTransform) []*fspb.DocumentTransform_FieldTransform {
	return t
}

func st(fieldPath string) *fspb.DocumentTransform_FieldTransform {
	return &fspb.DocumentTransform_FieldTransform{
		FieldPath: fieldPath,
		TransformType: &fspb.DocumentTransform_FieldTransform_SetToServerValue{
			fspb.DocumentTransform_FieldTransform_REQUEST_TIME,
		},
	}
}

func arrayUnion(fieldPath string, elems ...int) *fspb.DocumentTransform_FieldTransform {
	var i []*fspb.Value
	for _, e := range elems {
		i = append(i, &fspb.Value{ValueType: &fspb.Value_IntegerValue{int64(e)}})
	}
	return &fspb.DocumentTransform_FieldTransform{
		FieldPath: fieldPath,
		TransformType: &fspb.DocumentTransform_FieldTransform_AppendMissingElements{
			AppendMissingElements: &fspb.ArrayValue{Values: i},
		},
	}
}

func arrayRemove(fieldPath string, elems ...int) *fspb.DocumentTransform_FieldTransform {
	var i []*fspb.Value
	for _, e := range elems {
		i = append(i, &fspb.Value{ValueType: &fspb.Value_IntegerValue{int64(e)}})
	}
	return &fspb.DocumentTransform_FieldTransform{
		FieldPath: fieldPath,
		TransformType: &fspb.DocumentTransform_FieldTransform_RemoveAllFromArray{
			RemoveAllFromArray: &fspb.ArrayValue{Values: i},
		},
	}
}
