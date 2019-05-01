package swagvalidator_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/miketonks/swag"
	"github.com/miketonks/swag/endpoint"
	"github.com/miketonks/swag/swagger"
	"github.com/stretchr/testify/assert"

	sv "swag-validator"
)

type payload struct {
	FormatString     string          `json:"format_str" binding:"required" format:"uuid"`
	FormatStringArr  []string        `json:"format_str_arr" binding:"required" format:"uuid"`
	MinLenString     string          `json:"min_len_str,omitempty" min_length:"5"`
	MinLenStringArr  []string        `json:"min_len_str_arr,omitempty" min_length:"5"`
	MaxLenString     string          `json:"max_len_str,omitempty" max_length:"7"`
	MaxLenStringArr  []string        `json:"max_len_str_arr,omitempty" max_length:"7"`
	EnumString       string          `json:"enum_str,omitempty" enum:"Foo,Bar"`
	EnumStringArr    []string        `json:"enum_str_arr,omitempty" enum:"Foo,Bar"`
	PatternString    string          `json:"pattern_str,omitempty" pattern:"^test\\d$"`
	PatternStringArr []string        `json:"pattern_str_arr,omitempty" pattern:"^test\\d$"`
	RawJSON          json.RawMessage `json:"raw_json,omitempty"`
	Minimum          int             `json:"minimum,omitempty" minimum:"5"`
	Maximum          int             `json:"maximum,omitempty" maximum:"1"`
}

var testUUID = "00000000-0000-0000-0000-000000000000"

func TestSwaggerValidator(t *testing.T) {

	testTable := []struct {
		name             string
		description      string
		in               payload
		expectedStatus   int
		expectedResponse map[string]interface{}
	}{
		{
			description:    "Scalar uuid tag with non-uuid value",
			in:             payload{FormatString: "not-a-uuid"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.format_str": "Does not match format 'uuid'",
			},
		},
		{
			description:      "Scalar uuid tag with uuid value",
			in:               payload{FormatString: testUUID},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "UUID format tag on an array of strings with non-uuid values",
			in:             payload{FormatStringArr: []string{"not-a-uuid"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.format_str_arr.0": "Does not match format 'uuid'",
			},
		},
		{
			description:      "Testing uuid format tag on an array of strings with uuid values",
			in:               payload{FormatStringArr: []string{testUUID}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "Testing min len string tag with a string shorter than minimum required length",
			in:             payload{MinLenString: "1234"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.min_len_str": "String length must be greater than or equal to 5",
			},
		},
		{
			description:      "Testing min len string tag with a string longer than minimum required length",
			in:               payload{MinLenString: "123456"},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description: `Testing min len string tag with an array of strings where 
					at least one entry is shorter than minimum required length`,
			in:             payload{MinLenStringArr: []string{"1234"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.min_len_str_arr.0": "String length must be greater than or equal to 5",
			},
		},
		{
			description: `Testing min len string tag with an array of strings where 
					all entries are greater than or equal to minimum required length`,
			in:               payload{MinLenStringArr: []string{"12345", "123456"}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "Testing max len string tag with a string longer than maximum allowed length",
			in:             payload{MaxLenString: "12345678"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.max_len_str": "String length must be less than or equal to 7",
			},
		},
		{
			description:      "Testing max len string tag with a string shorter than maximum allowed length",
			in:               payload{MaxLenString: "123456"},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description: `Testing max len string tag with an array of strings where 
					at least one entry is longer than maximum allowed length`,
			in:             payload{MaxLenStringArr: []string{"12345678"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.max_len_str_arr.0": "String length must be less than or equal to 7",
			},
		},
		{
			description: `Testing max len string tag with an array of strings where 
					all entries are shorter than or equal to maxmimum allowed length`,
			in:               payload{MaxLenStringArr: []string{"123456", "1234567"}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "Testing enum string tag with a prohibited string value",
			in:             payload{EnumString: "test"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.enum_str": "body.enum_str must be one of the following: \"Foo\", \"Bar\"",
			},
		},
		{
			description:      "Testing enum string tag with an allowed string value",
			in:               payload{EnumString: "Foo"},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "Testing enum string tag on an array of strings where at least one value is prohibited",
			in:             payload{EnumStringArr: []string{"test"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.enum_str_arr.0": "body.enum_str_arr.0 must be one of the following: \"Bar\"",
			},
		},
		{
			description:      `Testing enum string tag on an array of strings where all values are allowed`,
			in:               payload{EnumStringArr: []string{"Bar"}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "Testing pattern string tag with a value that does not match the pattern",
			in:             payload{PatternString: "test"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.pattern_str": "Does not match pattern '^test\\d$'",
			},
		},
		{
			description:      "Testing pattern string tag with a value that does match the pattern",
			in:               payload{PatternString: "test1"},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description: `Testing pattern string tag on an array of strings 
					where at least one value does not match the pattern`,
			in:             payload{PatternStringArr: []string{"test"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.pattern_str_arr.0": "Does not match pattern '^test\\d$'",
			},
		},
		{
			description: `Testing pattern string tag on an array of strings 
					where at all values match the pattern`,
			in:               payload{PatternStringArr: []string{"test1", "test2", "test3"}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
	}

	url := "/validate-test/{test_id}"

	api := swag.New(swag.Endpoints(endpoint.New("POST", url, "Test the validator",
		endpoint.Handler(testHandler),
		endpoint.QueryMap(map[string]swagger.Parameter{
			"page": {
				Type:        "integer",
				Description: "page number to return",
			},
			"per_page": {
				Type:        "integer",
				Description: "Number of records per page",
			},
		}),
		endpoint.Body(payload{}, "Validation body", true),
		endpoint.Path("test_id", "string", "uuid", "group id"),
	)))

	sw, _ := api.RenderJSON()
	fmt.Println(string(sw))
	r := gin.Default()
	r.Use(sv.SwaggerValidator(api))
	api.Walk(func(path string, endpoint *swagger.Endpoint) {
		h := endpoint.Handler.(func(c *gin.Context))
		path = swag.ColonPath(path)

		r.Handle(endpoint.Method, path, h)
	})

	r.Run()
	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {

			w := httptest.NewRecorder()
			req := preparePostRequest(url, tt.in)
			r.ServeHTTP(w, req)

			var body map[string]interface{}

			if w.Body != nil && w.Body.String() != "" {
				err := json.Unmarshal(w.Body.Bytes(), &body)
				if err != nil {
					panic(fmt.Sprintf("Failed to unmarshal body while running test: %q. Error: %s", tt.name, err))
				}

				assert.Equal(t, tt.expectedResponse, body["details"])
			}

			assert.Equal(t, tt.expectedStatus, w.Code)

		})
	}
}

func preparePostRequest(url string, body payload) *http.Request {
	buff, err := json.Marshal(body)
	if err != nil {
		log.Fatalf("Failed to marshal the body: %s", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(buff))
	if err != nil {
		log.Fatalf("Error preparing request: %s", err)
	}

	return req
}

func testHandler(c *gin.Context) {

}
