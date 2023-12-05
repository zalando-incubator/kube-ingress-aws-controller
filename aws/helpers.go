package aws

import "os"

func mustRead(testFile string) string {
	buf, err := os.ReadFile("testdata/" + testFile)
	if err != nil {
		panic(err)
	}
	return string(buf)
}
