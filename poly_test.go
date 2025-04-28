package poly_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/clickermonkey/poly"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type Job interface {
	Do() string
}

type EmailJob struct {
	Message string `json:"message"`
}

func (e EmailJob) Do() string {
	return e.Message
}

type SaveJob struct{}

func (s SaveJob) Do() string {
	return "saving"
}

type StateJob struct {
	Done int `json:"done"`
}

func (s *StateJob) Do() string {
	s.Done++
	return fmt.Sprintf("Do() #%d", s.Done)
}

type HasJob struct {
	Job poly.T[Job] `json:"job"`
}

type HasPointerJob struct {
	Job *poly.T[Job] `json:"job,omitempty"`
}

func TestHappy(t *testing.T) {
	testCases := []struct {
		name        string
		setup       func()
		encode      any
		encodedJSON string
		encodedYAML string
		decode      func() any
		decodedTest func(decoded any, t *testing.T)
	}{
		{
			name: "simple specified value",
			setup: func() {
				poly.Register[SaveJob]("save")
				poly.Register[EmailJob]("email")
			},
			encode: HasJob{
				Job: poly.C[Job](EmailJob{Message: "Hello World!"}),
			},
			encodedJSON: `{"job":["email",{"message":"Hello World!"}]}`,
			encodedYAML: "job:\n  - email\n  - message: Hello World!\n",
			decode:      func() any { return &HasJob{} },
			decodedTest: func(decoded any, t *testing.T) {
				hj := decoded.(*HasJob)
				if done := hj.Job.Value.Do(); done != "Hello World!" {
					t.Fatalf("parsing failure: %s", done)
				}
			},
		},
		{
			name: "specialized specified value",
			setup: func() {
				poly.Register[SaveJob]("job-save")
				poly.Register[EmailJob]("job-email")

				poly.RegisterSpecialized[Job, SaveJob]("save")
				poly.RegisterSpecialized[Job, EmailJob]("email")
			},
			encode: HasJob{
				Job: poly.C[Job](EmailJob{Message: "Hello World!"}),
			},
			encodedJSON: `{"job":["email",{"message":"Hello World!"}]}`,
			encodedYAML: "job:\n  - email\n  - message: Hello World!\n",
			decode:      func() any { return &HasJob{} },
			decodedTest: func(decoded any, t *testing.T) {
				hj := decoded.(*HasJob)
				if done := hj.Job.Value.Do(); done != "Hello World!" {
					t.Fatalf("parsing failure: %s", done)
				}
			},
		},
		{
			name: "pointer value",
			setup: func() {
				poly.Register[*StateJob]("state")
			},
			encode: HasJob{
				Job: poly.C[Job](&StateJob{Done: 1}),
			},
			encodedJSON: `{"job":["state",{"done":1}]}`,
			encodedYAML: "job:\n  - state\n  - done: 1\n",
			decode:      func() any { return &HasJob{} },
			decodedTest: func(decoded any, t *testing.T) {
				hj := decoded.(*HasJob)
				if done := hj.Job.Value.Do(); done != "Do() #2" {
					t.Fatalf("parsing failure: %s", done)
				}
			},
		},
		{
			name: "pointer type",
			setup: func() {
				poly.Register[*StateJob]("state")
			},
			encode: HasPointerJob{
				Job: poly.P[Job](&StateJob{Done: 1}),
			},
			encodedJSON: `{"job":["state",{"done":1}]}`,
			encodedYAML: "job:\n  - state\n  - done: 1\n",
			decode:      func() any { return &HasPointerJob{} },
			decodedTest: func(decoded any, t *testing.T) {
				hj := decoded.(*HasPointerJob)
				if done := hj.Job.Value.Do(); done != "Do() #2" {
					t.Fatalf("parsing failure: %s", done)
				}
			},
		},
		{
			name: "no value",
			setup: func() {
				poly.Register[SaveJob]("save")
				poly.Register[EmailJob]("email")
			},
			encode:      HasJob{},
			encodedJSON: `{"job":[]}`,
			encodedYAML: "job: null\n",
			decode:      func() any { return &HasJob{} },
			decodedTest: func(decoded any, t *testing.T) {
				hj := decoded.(*HasJob)
				if hj.Job.Value != nil {
					t.Fatal("parsing failure")
				}
			},
		},
		{
			name: "no pointer",
			setup: func() {
				poly.Register[SaveJob]("save")
				poly.Register[EmailJob]("email")
			},
			encode:      HasPointerJob{},
			encodedJSON: `{}`,
			encodedYAML: "job: null\n",
			decode:      func() any { return &HasPointerJob{} },
			decodedTest: func(decoded any, t *testing.T) {
				hj := decoded.(*HasPointerJob)
				if hj.Job != nil {
					t.Fatal("parsing failure")
				}
			},
		},
		{
			name: "raw specified value",
			setup: func() {
				poly.Register[SaveJob]("save")
				poly.Register[EmailJob]("email")
			},
			encode:      poly.C[Job](EmailJob{Message: "Hello World!"}),
			encodedJSON: `["email",{"message":"Hello World!"}]`,
			encodedYAML: "- email\n- message: Hello World!\n",
			decode:      func() any { return &poly.T[Job]{} },
			decodedTest: func(decoded any, t *testing.T) {
				hj := decoded.(*poly.T[Job])
				if done := hj.Value.Do(); done != "Hello World!" {
					t.Fatalf("parsing failure: %s", done)
				}
			},
		},
		{
			name: "raw no value",
			setup: func() {
				poly.Register[SaveJob]("save")
				poly.Register[EmailJob]("email")
			},
			encode:      poly.T[Job]{},
			encodedJSON: `[]`,
			encodedYAML: "null\n",
			decode:      func() any { return &poly.T[Job]{} },
			decodedTest: func(decoded any, t *testing.T) {
				hj := decoded.(*poly.T[Job])
				if hj.Value != nil {
					t.Fatal("parsing failure")
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			poly.Reset()

			if testCase.setup != nil {
				testCase.setup()
			}

			actualJSON, err := json.Marshal(testCase.encode)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, testCase.encodedJSON, string(actualJSON), "json mismatch")

			yamlOut := strings.Builder{}
			yamlEnc := yaml.NewEncoder(&yamlOut)
			yamlEnc.SetIndent(2)
			err = yamlEnc.Encode(testCase.encode)
			if err != nil {
				t.Fatal(err)
			}
			actualYAML := yamlOut.String()
			assert.Equal(t, testCase.encodedYAML, actualYAML, "yaml mismatch")

			if testCase.decodedTest != nil && testCase.decode != nil {
				decodedJSON := testCase.decode()
				err = json.Unmarshal(actualJSON, decodedJSON)
				if err != nil {
					t.Fatal(err)
				}

				testCase.decodedTest(decodedJSON, t)

				decodedYAML := testCase.decode()
				err = yaml.Unmarshal([]byte(actualYAML), decodedYAML)
				if err != nil {
					t.Fatal(err)
				}

				testCase.decodedTest(decodedYAML, t)
			}
		})
	}
}
