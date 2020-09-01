package server

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPath(t *testing.T) {
	testPath, err := ioutil.TempDir("./", "")
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(testPath)
		require.NoError(t, err)
	}()
	path, err := getPath(testPath, 123)
	require.NoError(t, err)
	require.Equal(t, testPath+"/BlockStorage_100000/dump-block-1000.json", path)

	path, err = getPath(testPath, 1230)
	require.NoError(t, err)
	require.Equal(t, testPath+"/BlockStorage_100000/dump-block-2000.json", path)

	path, err = getPath(testPath, 123000)
	require.NoError(t, err)
	require.Equal(t, testPath+"/BlockStorage_200000/dump-block-123000.json", path)
}
