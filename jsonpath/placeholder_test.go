package jsonpath_test

import (
	"testing"

	"github.com/kubemod/kubemod/jsonpath"
)

func TestWildcardsExtension(t *testing.T) {
	tests := []jsonpathTest{
		{
			name: "constant",
			path: `{"x" : "y", "z" : "a"}`,
			data: `"hey"`,
			want: obj{"x": "y", "z": "a"},
		},
		{
			name: "root",
			path: `{"x" : "y", "z" : $}`,
			data: `"hey"`,
			want: obj{"x": "y", "z": "hey"},
		},
		{
			name: "range array",
			path: `{#0: $[2:6].a}`,
			data: `[55,41,70,{"a":"bb"}]`,
			want: obj{
				"3": "bb",
			},
		},
		{
			name: "range object", //no range over objects
			path: `{#0: $[2:6].a}`,
			data: `{"3":{"a":"aa"}}`,
			want: obj{},
		},
		{
			name: "range multi match",
			path: `{#0: $[2:6].a}`,
			data: `[{"a":"xx"},41,{"a":"b1"},{"a":"b2"},55,{"a":"b3"},{"a":"x2"} ]`,
			want: obj{
				"2": "b1",
				"3": "b2",
				"5": "b3",
			},
		},
		{
			name: "range all",
			path: `{#0: $[:]}`,
			data: `[55,41,70,{"a":"bb"}]`,
			want: obj{
				"0": 55.,
				"1": 41.,
				"2": 70.,
				"3": obj{"a": "bb"},
			},
		},
		{
			name: "range all even",
			path: `{#0: $[::2]}`,
			data: `[55,41,70,{"a":"bb"}]`,
			want: obj{
				"0": 55.,
				"2": 70.,
			},
		},
		{
			name: "range all even reverse",
			path: `{#0: $[::-2]}`,
			data: `[55,41,70,{"a":"bb"}]`,
			want: obj{
				"1": 41.,
				"3": obj{"a": "bb"},
			},
		},
		{
			name: "union wildcard array first",
			path: `{#0: $[1, 3].*}`,
			data: `[55,{"a":"1a"},70,{"b":"bb"}]`,
			want: obj{
				"1": "1a",
				"3": "bb",
			},
		},
		{
			name: "union wildcard array second",
			path: `{#1: $[1, 3].*}`,
			data: `[55,{"a":"1a"},70,{"b":"bb", "c":"cc"}]`,
			want: obj{
				"a": "1a",
				"b": "bb",
				"c": "cc",
			},
		},
		{
			name: "union wildcard object first",
			path: `{#0: $[1, 3].*}`,
			data: `{"3":{"a":"3a"}, "1":{"7":"1a"}, "x":{"a":"bb"}}`,
			want: obj{
				"1": "1a",
				"3": "3a",
			},
		},
		{
			name: "union wildcard object second",
			path: `{#1: $[1, 3].*}`,
			data: `{"3":{"a":"3a"}, "1":{"7":"1a"}, "x":{"a":"bb"}}`,
			want: obj{
				"7": "1a",
				"a": "3a",
			},
		},
		{
			name: "union bracket wildcard object first",
			path: "{#0: $[1, 3][*]}",
			data: `{"3":{"a":"3a"}, "1":{"7":"1a"}, "x":{"a":"bb"}}`,
			want: obj{
				"1": "1a",
				"3": "3a",
			},
		},
		{
			name: "union bracket wildcard object second",
			path: "{#1: $[1, 3][*]}",
			data: `{"3":{"a":"3a"}, "1":{"7":"1a"}, "x":{"a":"bb"}}`,
			want: obj{
				"7": "1a",
				"a": "3a",
			},
		},
		{
			name: "mapper",
			path: "{#: $..x}",
			data: `{
							"a" : {"x" : 1},
							"b" : [{"x" : 2}, {"y" : 3}],
							"x" : 4
						}`,
			want: obj{
				`$["a"]`:      1.,
				`$["b"]["0"]`: 2.,
				`$`:           4.,
			},
		},
		{
			name: "mapper union",
			path: `{#1: $..["x", "a"]}`,
			data: `{
							"a" : {"x" : 1}
						}`,
			want: obj{
				`a`: obj{"x": 1.},
				`x`: 1.,
			},
		},
		{
			name: "mapper filter",
			path: `{#1: $..[?@.a=="aa"]}`,
			data: `{"1":{"a":"aa", "b":[1 ,2, 3]}, "3":{}, "x":{"7":"bb"}, "y":{"a":"bb"}}`,
			want: obj{
				"1": obj{"a": "aa", "b": arr{1., 2., 3.}},
			},
		},
		{
			name: "brackets",
			path: `{ #0 : $[*]["line-rx"]}`,
			data: `{"1":{"line-rx":"aa", "b":[1 ,2, 3]}, "3":{}, "x":{"line-rx":"bb"}, "y":{"a":"bb"}}`,
			want: obj{
				"1": "aa",
				"x": "bb",
			},
		},
	}
	for _, tt := range tests {
		tt.lang = jsonpath.PlaceholderExtension()
		t.Run(tt.name, tt.test)
	}
}
