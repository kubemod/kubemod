package jsonpath_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/PaesslerAG/gval"
	"github.com/kubemod/kubemod/jsonpath"
)

type jsonpathTest struct {
	name         string
	path         string
	data         string
	lang         gval.Language
	reorder      bool
	want         interface{}
	wantErr      bool
	wantParseErr bool
}

type obj = map[string]interface{}
type arr = []interface{}

func TestJsonPath(t *testing.T) {

	tests := []jsonpathTest{
		{
			name: "root string",
			path: "$",
			data: `"hey"`,
			want: "hey",
		},
		{
			name: "root object",
			path: "$",
			data: `{"a":"aa"}`,
			want: obj{"a": "aa"},
		},
		{
			name: "simple select array",
			path: "$[1]",
			data: `[7, "hey"]`,
			want: "hey",
		},
		{
			name:    "negativ select array",
			path:    "$[-1]",
			data:    `[7, "hey"]`,
			wantErr: true,
		},
		{
			name: "simple select object",
			path: "$[1]",
			data: `{"1":"aa"}`,
			want: "aa",
		},
		{
			name:    "simple select out of bounds",
			path:    "$[1]",
			data:    `["hey"]`,
			wantErr: true,
		},
		{
			name: "simple select unknown key",
			path: "$[1]",
			data: `{"2":"aa"}`,
			want: jsonpath.Undefined,
		},
		{
			name: "deep select unknown key",
			path: "$.a.x",
			data: `{"a":{"b":12}}`,
			want: jsonpath.Undefined,
		},
		{
			name: "deeper select unknown key",
			path: "$.a.x.y",
			data: `{"a":{"b":12}}`,
			want: jsonpath.Undefined,
		},
		{
			name: "select array",
			path: "$[3].a",
			data: `[55,41,70,{"a":"bb"}]`,
			want: "bb",
		},
		{
			name: "select object",
			path: "$[3].a",
			data: `{"3":{"a":"aa"}}`,
			want: "aa",
		},
		{
			name: "range array",
			path: "$[2:6].a",
			data: `[55,41,70,{"a":"bb"}]`,
			want: arr{"bb"},
		},
		{
			name: "range object", //no range over objects
			path: "$[2:6].a",
			data: `{"3":{"a":"aa"}}`,
			want: arr{},
		},
		{
			name: "range multi match",
			path: "$[2:6].a",
			data: `[{"a":"xx"},41,{"a":"b1"},{"a":"b2"},55,{"a":"b3"},{"a":"x2"} ]`,
			want: arr{
				"b1",
				"b2",
				"b3",
			},
		},
		{
			name: "range all",
			path: "$[:]",
			data: `[55,41,70,{"a":"bb"}]`,
			want: arr{
				55.,
				41.,
				70.,
				obj{"a": "bb"},
			},
		},
		{
			name: "range all even",
			path: "$[::2]",
			data: `[55,41,70,{"a":"bb"}]`,
			want: arr{
				55.,
				70.,
			},
		},
		{
			name: "range all even reverse",
			path: "$[::-2]",
			data: `[55,41,70,{"a":"bb"}]`,
			want: arr{
				obj{"a": "bb"},
				41.,
			},
		},
		{
			name: "range reverse",
			path: "$[2:6:-1].a",
			data: `[55,41,70,{"a":"bb"}]`,
			want: arr{
				"bb",
			},
		},
		{
			name: "range reverse multi match",
			path: "$[2:6:-1].a",
			data: `[{"a":"xx"},41,{"a":"b1"},{"a":"b2"},55,{"a":"b3"},{"a":"x2"} ]`,
			want: arr{
				"b3",
				"b2",
				"b1",
			},
		},
		{
			name: "range even selection",
			path: "$[2:6:2].a",
			data: `[55,41,70,{"a":"bb"}]`,
			want: arr{},
		},
		{
			name: "range even multi match selection",
			path: "$[2:6:2].a",
			data: `[{"a":"xx"},41,{"a":"b1"},{"a":"b2"},{"a":"b3"},{"a":"x2"} ]`,
			want: arr{
				"b1",
				"b3",
			},
		},
		{
			name: "current",
			path: "$.a[@.max]",
			data: `{"a":{"max":"3a", "3a":"aa"}, "1":{"a":"1a"}, "x":{"7":"bb"}}`,
			want: "aa",
		},
		{
			name: "union array",
			path: "$[1, 3].a",
			data: `[55,{"a":"1a"},70,{"a":"bb"}]`,
			want: arr{
				"1a",
				"bb",
			},
		},
		{
			name: "negativ union array",
			path: "$[1, -5, 3].a",
			data: `[55,{"a":"1a"},70,{"a":"bb"}]`,
			want: arr{
				"1a",
				"bb",
			},
		},
		{
			name: "union object",
			path: "$[1, 3].a",
			data: `{"3":{"a":"3a"}, "1":{"a":"1a"}, "x":{"7":"bb"}}`,
			want: arr{
				"1a",
				"3a",
			},
		},
		{
			name: "union array partilly matched",
			path: "$[1, 3].a",
			data: `[55,41,70,{"a":"bb"}]`,
			want: arr{
				"bb",
			},
		},
		{
			name: "union object partially matched",
			path: "$[1, 3].a",
			data: `{"1":{"a":"aa"}, "3":{}, "x":{"7":"bb"}}`,
			want: arr{
				"aa",
			},
		},
		{
			name: "union wildcard array",
			path: "$[1, 3].*",
			data: `[55,{"a":"1a"},70,{"b":"bb", "c":"cc"}]`,
			want: arr{
				"1a",
				"bb",
				"cc",
			},
			reorder: true,
		},
		{
			name: "union wildcard object",
			path: "$[1, 3].*",
			data: `{"3":{"a":"3a"}, "1":{"7":"1a"}, "x":{"a":"bb"}}`,
			want: arr{
				"1a",
				"3a",
			},
		},
		{
			name: "union wildcard array partilly matched",
			path: "$[1, 3].*",
			data: `[55,41,70,{"a":"bb"}]`,
			want: arr{
				"bb",
			},
		},
		{
			name: "union wildcard object partilly matched",
			path: "$[1, 3].*",
			data: `{"1":{"a":"aa", "7":"cc"}, "3":{}, "x":{"7":"bb"}}`,
			want: arr{
				"aa",
				"cc",
			},
			reorder: true,
		},
		{
			name: "union bracket wildcard array",
			path: "$[1, 3][*]",
			data: `[55,{"a":"1a"},70,{"b":"bb", "c":"cc"}]`,
			want: arr{
				"1a",
				"bb",
				"cc",
			},
			reorder: true,
		},
		{
			name: "union bracket wildcard object",
			path: "$[1, 3][*]",
			data: `{"3":{"a":"3a"}, "1":{"7":"1a"}, "x":{"a":"bb"}}`,
			want: arr{
				"1a",
				"3a",
			},
		},
		{
			name:         "incomplete",
			path:         "$[3].",
			wantParseErr: true,
		},
		{
			name:         "mixed bracket",
			path:         "$[3,5:1].",
			wantParseErr: true,
		},
		{
			name: "mapper",
			path: "$..x",
			data: `{
					"a" : {"x" : 1},
					"b" : [{"x" : 2}, {"y" : 3}],
					"x" : 4
				}`,
			want: arr{
				1.,
				2.,
				4.,
			},
			reorder: true,
		},
		{
			name: "mapper union",
			path: `$..["x", "a"]`,
			data: `{
					"a" : {"x" : 1},
					"b" : [{"x" : 2}, {"y" : 3}],
					"x" : 4
				}`,
			want: arr{
				1.,
				2.,
				4.,
				obj{"x": 1.},
			},
			reorder: true,
		},
		{
			name: "mapper wildcard",
			path: `$..*`,
			data: `{"1":{"a":"aa", "b":[1 ,2, 3]}, "3":{}, "x":{"7":"bb"}}`,
			want: arr{
				1.,
				2.,
				3.,
				"aa",
				"bb",
				arr{1., 2., 3.},
				obj{},
				obj{"7": "bb"},
				obj{"a": "aa", "b": arr{1., 2., 3.}},
			},
			reorder: true,
		},
		{
			name: "mapper filter true",
			path: `$..[?true]`,
			data: `{"1":{"a":"aa", "b":[1 ,2, 3]}, "3":{}, "x":{"7":"bb"}}`,
			want: arr{
				1.,
				2.,
				3.,
				"aa",
				"bb",
				arr{1., 2., 3.},
				obj{},
				obj{"7": "bb"},
				obj{"a": "aa", "b": arr{1., 2., 3.}},
			},
			reorder: true,
		},
		{
			name: "mapper filter a=aa",
			path: `$..[?@.a=="aa"]`,
			data: `{"1":{"a":"aa", "b":[1 ,2, 3]}, "3":{}, "x":{"7":"bb"}, "y":{"a":"bb"}}`,
			want: arr{
				obj{"a": "aa", "b": arr{1., 2., 3.}},
			},
		},
		{
			name: "mapper filter (a=aa)",
			path: `$..[?(@.a=="aa")]`,
			data: `{"1":{"a":"aa", "b":[1 ,2, 3]}, "3":{}, "x":{"7":"bb"}, "y":{"a":"bb"}}`,
			want: arr{
				obj{"a": "aa", "b": arr{1., 2., 3.}},
			},
		},
		{
			name: "key value",
			path: `$[?(@.key=="x")].value`,
			data: `[{"key": "x","value":"a"},{"key": "y","value":"b"}]`,
			want: arr{
				"a",
			},
		},
		{
			name: "script",
			path: `$.*.value(@=="a")`,
			data: `[{"key": "x","value":"a"},{"key": "y","value":"b"}]`,
			want: arr{
				true,
				false,
			},
		},
		{
			name: "mapper script",
			path: `$..(@=="a")`,
			data: `[{"key": "x","value":"a"},{"key": "y","value":"b"}]`,
			want: arr{
				false,
				false,
				false,
				false,
				false,
				false,
				true,
			},
			reorder: true,
		},
		{
			name: "mapper select script",
			path: `$.abc.f..["x"](@ == "1")`,
			data: `{
					"abc":{
						"d":[
							"1",
							"1"
						],
						"f":{
							"a":{
								"x":"1"
							},
							"b":{
								"x":"1"
							},
							"c":{
								"x":"xx"
							}
						}
					}
				}`,
			want: arr{
				false,
				true,
				true,
			},
			reorder: true,
		},
		{
			name: "float equal",
			path: `$.a == 1.23`,
			data: `{"a":1.23, "b":2}`,
			want: true,
		},
		{
			name: "ending star",
			path: `$.welcome.message[*]`,
			data: `{"welcome":{"message":["Good Morning", "Hello World!"]}}`,
			want: arr{"Good Morning", "Hello World!"},
		},
	}
	for _, tt := range tests {
		tt.lang = jsonpath.Language()
		t.Run(tt.name, tt.test)
	}
}

func (tt jsonpathTest) test(t *testing.T) {
	get, err := tt.lang.NewEvaluable(tt.path)
	if (err != nil) != tt.wantParseErr {
		t.Fatalf("New() error = %v, wantErr %v", err, tt.wantErr)
	}
	if tt.wantParseErr {
		return
	}
	var v interface{}
	err = json.Unmarshal([]byte(tt.data), &v)
	if err != nil {
		t.Fatalf("could not parse json input: %v", err)
	}
	got, err := get(context.Background(), v)

	if tt.wantErr {
		if err == nil {
			t.Errorf("expected error %v but got %v", tt.wantErr, got)
			return
		}
		return
	}

	if err != nil {
		t.Errorf("JSONPath(%s) error = %v", tt.path, err)
		return
	}

	if tt.reorder {
		reorder(got.(arr))
	}

	if jsonpath.IsUndefined(tt.want) && jsonpath.IsUndefined(got) {
		return
	}

	if !reflect.DeepEqual(got, tt.want) {
		t.Fatalf("expected %v, but got %v", tt.want, got)
	}
}

func reorder(sl []interface{}) {
	sort.Slice(sl, func(i, j int) bool {
		a := sl[i]
		b := sl[j]
		if reflect.TypeOf(a) != reflect.TypeOf(b) {
			return typeOrder(a) < typeOrder(b)
		}

		switch a := a.(type) {
		case string:
			return a < b.(string)
		case float64:
			return a < b.(float64)
		case bool:
			return !a || b.(bool)
		case arr:
			return len(a) < len(b.(arr))
		case obj:
			return len(a) < len(b.(obj))
		default:
			panic(fmt.Errorf("unknown type %T", a))
		}
	})
}

func typeOrder(o interface{}) int {
	switch o.(type) {
	case bool:
		return 0
	case float64:
		return 1
	case string:
		return 2
	case arr:
		return 3
	case obj:
		return 4

	default:
		panic(fmt.Errorf("unknown type %T", o))
	}
}
