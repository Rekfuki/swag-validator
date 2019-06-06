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
	swag "github.com/miketonks/swag"
	"github.com/miketonks/swag/endpoint"
	"github.com/miketonks/swag/swagger"
	"github.com/stretchr/testify/assert"

	sv "github.com/miketonks/swag-validator"
)

type nested struct {
	Foo string `json:"foo" binding:"required"`
}

type payload struct {
	FormatString     string   `json:"format_str" format:"uuid"`
	FormatStringArr  []string `json:"format_str_arr" format:"uuid"`
	MinLenString     string   `json:"min_len_str,omitempty" min_length:"5"`
	MinLenStringArr  []string `json:"min_len_str_arr,omitempty" min_length:"5"`
	MaxLenString     string   `json:"max_len_str,omitempty" max_length:"7"`
	MaxLenStringArr  []string `json:"max_len_str_arr,omitempty" max_length:"7"`
	EnumString       string   `json:"enum_str,omitempty" enum:"Foo,Bar"`
	EnumStringArr    []string `json:"enum_str_arr,omitempty" enum:"Foo,Bar"`
	PatternString    string   `json:"pattern_str,omitempty" pattern:"^test\\d$"`
	PatternStringArr []string `json:"pattern_str_arr,omitempty" pattern:"^test\\d$"`
	Minimum          int      `json:"minimum,omitempty" minimum:"5"`
	Maximum          int      `json:"maximum,omitempty" maximum:"1"`
	Nested           nested   `json:"nested"`
}

var testUUID = "00000000-0000-0000-0000-000000000000"

func TestSwaggerValidator(t *testing.T) {

	testTable := []struct {
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
			description:    "Non-UUID string in a UUID array",
			in:             payload{FormatStringArr: []string{"not-a-uuid"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.format_str_arr.0": "Does not match format 'uuid'",
			},
		},
		{
			description:      "UUID strings in a UUID array",
			in:               payload{FormatStringArr: []string{testUUID}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "String shorter than minimum required",
			in:             payload{MinLenString: "1234"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.min_len_str": "String length must be greater than or equal to 5",
			},
		},
		{
			description:      "String longer than minimum required",
			in:               payload{MinLenString: "123456"},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "String in an array shorter than minimum required",
			in:             payload{MinLenStringArr: []string{"1234"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.min_len_str_arr.0": "String length must be greater than or equal to 5",
			},
		},
		{
			description:      "Strings in an array longer than minimum required",
			in:               payload{MinLenStringArr: []string{"12345", "123456"}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "String longer than maximum allowed",
			in:             payload{MaxLenString: "12345678"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.max_len_str": "String length must be less than or equal to 7",
			},
		},
		{
			description:      "String shoter or equal to maximum allowed",
			in:               payload{MaxLenString: "123456"},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    `String in an array longer than maximum allowed`,
			in:             payload{MaxLenStringArr: []string{"12345678"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.max_len_str_arr.0": "String length must be less than or equal to 7",
			},
		},
		{
			description:      "Strings in an array shorter than or euqal to maximum allowed",
			in:               payload{MaxLenStringArr: []string{"123456", "1234567"}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "String does not match enumaration",
			in:             payload{EnumString: "test"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.enum_str": "body.enum_str must be one of the following: \"Foo\", \"Bar\"",
			},
		},
		{
			description:      "String matches enumeration",
			in:               payload{EnumString: "Foo"},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "String in an array does not match enumeration",
			in:             payload{EnumStringArr: []string{"test"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.enum_str_arr.0": "body.enum_str_arr.0 must be one of the following: \"Bar\"",
			},
		},
		{
			description:      `Strings in an arrya match enumeration`,
			in:               payload{EnumStringArr: []string{"Bar"}},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    "String does not match pattern",
			in:             payload{PatternString: "test"},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.pattern_str": "Does not match pattern '^test\\d$'",
			},
		},
		{
			description:      "String matches pattern",
			in:               payload{PatternString: "test1"},
			expectedStatus:   200,
			expectedResponse: nil,
		},
		{
			description:    `String in an array does not match pattern`,
			in:             payload{PatternStringArr: []string{"test"}},
			expectedStatus: 400,
			expectedResponse: map[string]interface{}{
				"body.pattern_str_arr.0": "Does not match pattern '^test\\d$'",
			},
		},
		{
			description:      "Strings in an array match pattern",
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

	for _, tt := range testTable {
		t.Run(tt.description, func(t *testing.T) {

			w := httptest.NewRecorder()
			req := preparePostRequest(url, tt.in)
			r.ServeHTTP(w, req)

			var body map[string]interface{}

			if w.Body != nil && w.Body.String() != "" {
				err := json.Unmarshal(w.Body.Bytes(), &body)
				if err != nil {
					panic(fmt.Sprintf("Failed to unmarshal body while running test: %q. Error: %s", tt.description, err))
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

func testHandler(c *gin.Context) {}
