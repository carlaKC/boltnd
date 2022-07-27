package offers

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

const formatStringTestJson = "format-string-test.json"

type offerFormatTestVector struct {
	Comment string `json:"comment"`
	Valid   bool   `json:"valid"`
	String  string `json:"string"`
}

// TestOfferStringEncoding tests decoding of the test vectors for offer strings.
// Note that the following test cases are missing:
//
// {
// "comment": "A complete string is valid",
// "valid": true,
// "string": "lno1qcp4256ypqpq86q2pucnq42ngssx2an9wfujqerp0y2pqun4wd68jtn00fkxzcnn9ehhyec6qgqsz83qfwdpl28qqmc78ymlvhmxcsywdk5wrjnj36jryg488qwlrnzyjczlqsp9nyu4phcg6dqhlhzgxagfu7zh3d9re0sqp9ts2yfugvnnm9gxkcnnnkdpa084a6t520h5zhkxsdnghvpukvd43lastpwuh73k29qsy"
// },
// {
// "comment": "+ can join anywhere",
// "valid": true,
// "string": "l+no1qcp4256ypqpq86q2pucnq42ngssx2an9wfujqerp0y2pqun4wd68jtn00fkxzcnn9ehhyec6qgqsz83qfwdpl28qqmc78ymlvhmxcsywdk5wrjnj36jryg488qwlrnzyjczlqsp9nyu4phcg6dqhlhzgxagfu7zh3d9re0sqp9ts2yfugvnnm9gxkcnnnkdpa084a6t520h5zhkxsdnghvpukvd43lastpwuh73k29qsy"
// },
// {
// "comment": "Multiple + can join",
// "valid": true,
// "string": "lno1qcp4256ypqpq+86q2pucnq42ngssx2an9wfujqerp0y2pqun4wd68jtn0+0fkxzcnn9ehhyec6qgqsz83qfwdpl28qqmc78ymlvhmxcsywdk5wrjnj36jryg488qwlrnzyjczlqsp9nyu4phcg6dqhlhzgxagfu7zh3d9re0+sqp9ts2yfugvnnm9gxkcnnnkdpa084a6t520h5zhkxsdnghvpukvd43lastpwuh73k29qs+y"
// },
// {
// "comment": "+ can be followed by whitespace",
// "valid": true,
// "string": "lno1qcp4256ypqpq+ 86q2pucnq42ngssx2an9wfujqerp0y2pqun4wd68jtn0+  0fkxzcnn9ehhyec6qgqsz83qfwdpl28qqmc78ymlvhmxcsywdk5wrjnj36jryg488qwlrnzyjczlqsp9nyu4phcg6dqhlhzgxagfu7zh3d9re0+\nsqp9ts2yfugvnnm9gxkcnnnkdpa084a6t520h5zhkxsdnghvpukvd43l+\r\nastpwuh73k29qs+\r  y"
// },
func TestOfferStringEncoding(t *testing.T) {
	vectorBytes, err := ioutil.ReadFile(formatStringTestJson)
	require.NoError(t, err, "read file")

	var testCases []*offerFormatTestVector
	require.NoError(t, json.Unmarshal(vectorBytes, &testCases))

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.Comment, func(t *testing.T) {
			_, err := DecodeOfferStr(testCase.String)
			require.Equal(t, testCase.Valid, err == nil,
				"error check: %v", err)
		})
	}
}
