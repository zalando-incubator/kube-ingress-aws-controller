package cloudformation

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"
)

var testFilePaths = []string{
	"examples/minimal.template",
	"examples/minimal_outputs.template",
	"examples/app.template",
	"examples/app-minimal.template",
	"examples/LAMP_Multi_AZ.template",
	"examples/LAMP_Single_Instance.template",
	"examples/Rails_Multi_AZ.template",
	"examples/Rails_Single_Instance.template",
	"examples/Windows_Roles_And_Features.template",
	"examples/Windows_Single_Server_Active_Directory.template",
	"examples/Windows_Single_Server_SharePoint_Foundation.template",
	"examples/WordPress_Chef.template",
	"examples/WordPress_Multi_AZ.template",
	"examples/WordPress_Single_Instance.template",
}

func TestRoundtrips(t *testing.T) {
	for _, testFilePath := range testFilePaths {
		buf, err := ioutil.ReadFile(testFilePath)
		if err != nil {
			panic(err)
		}
		testMarshalResourcesPieceByPiece(t, testFilePath, buf)
		//testOne(t, buf)
	}
}

func testMarshalResourcesPieceByPiece(t *testing.T, path string, input []byte) {
	v := map[string]interface{}{}
	err := json.Unmarshal(input, &v)
	resources := v["Resources"].(map[string]interface{})
	for name, resource := range resources {
		buf, _ := json.Marshal(resource)
		r := Resource{}
		err = json.Unmarshal(buf, &r)
		if err != nil {
			t.Errorf("marshal: %s %s: %s", path, name, err)
			return
		}
	}
}

func testOne(t *testing.T, input []byte) {
	templ := Template{}
	err := json.Unmarshal(input, &templ)
	if err != nil {
		t.Errorf("decode: %s", err)
		return
	}

	output, err := json.Marshal(templ)
	if err != nil {
		t.Errorf("marshal: %s", err)
		return
	}

	parsedInput := map[string]interface{}{}
	json.Unmarshal(input, &parsedInput)

	parsedOutput := map[string]interface{}{}
	json.Unmarshal(output, &parsedOutput)

	if !reflect.DeepEqual(parsedInput, parsedOutput) {
		t.Errorf("expected %#v, got %#v", parsedInput, parsedOutput)
	}
}
