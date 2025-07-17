package cloudformation

import (
	"encoding/json"
	"os"
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
		buf, err := os.ReadFile(testFilePath)
		if err != nil {
			panic(err)
		}
		testMarshalResourcesPieceByPiece(t, testFilePath, buf)
		//testOne(t, buf)
	}
}

func testMarshalResourcesPieceByPiece(t *testing.T, path string, input []byte) {
	v := map[string]interface{}{}
	_ = json.Unmarshal(input, &v)
	resources := v["Resources"].(map[string]interface{})
	for name, resource := range resources {
		buf, _ := json.Marshal(resource)
		r := Resource{}
		err := json.Unmarshal(buf, &r)
		if err != nil {
			t.Errorf("marshal: %s %s: %s", path, name, err)
			return
		}
	}
}
