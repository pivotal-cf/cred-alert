package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

func bindata_read(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	return buf.Bytes(), nil
}

var _web_templates_index_html = []byte("\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\x64\x92\x31\x6f\xe3\x30\x0c\x85\xf7\xfc\x0a\x9e\x6e\x8e\x14\x20\xc0\x0d\x07\xd9\xc3\xe5\xba\x36\x1d\xba\x74\x94\x2d\x36\x12\x2a\x4b\x81\xc4\x04\x70\x05\xff\xf7\xc2\x56\xea\x38\xee\x64\x9a\xe6\xe3\xfb\xf0\x68\xf9\xeb\xff\xf1\xf0\xfa\xf6\xf2\x04\x86\x3a\x57\x6f\xe4\xf8\x00\xa7\xfc\xa9\x62\xe8\x59\xbd\x01\x90\x06\x95\x1e\x0b\x00\x49\x96\x1c\xd6\x87\x88\x1a\x3d\x59\xe5\xe0\x10\x2e\x9e\xa0\xe9\xe1\x18\x4f\xca\xdb\x4f\x45\x36\x78\x29\xca\x5c\xd1\x38\xeb\x3f\x20\xa2\xab\x58\xa2\xde\x61\x32\x88\xc4\xc0\x44\x7c\xaf\x98\x21\x3a\xff\x15\xa2\xbf\x58\xde\x2b\x13\x82\x3a\xdb\xc4\xdb\xd0\x89\xf3\x25\xa2\xd8\xf1\x3f\x7c\x37\x95\xdb\xce\x7a\xde\xa6\x54\x80\xc4\x37\x91\x6c\x82\xee\x6f\x36\xda\x5e\xc1\xea\x8a\xb5\xc1\x13\x7a\x62\xa5\x3d\xe2\xef\x7f\x00\x27\xf8\xb7\x26\x36\xfb\x59\x40\xaa\x71\x08\xad\x53\x29\x55\x6c\x72\x2f\x9d\x7b\xb9\x6d\x42\xd4\x18\x51\xcf\x2e\xa3\xec\x9e\xd3\xdc\xa9\x57\xb1\x98\xf5\xf7\x35\xd9\xe3\xcc\xf8\x36\x2d\x9d\x3b\x39\x47\xe5\x4f\x08\x7c\xb9\x38\x0d\xc3\x02\x23\x3e\x7a\xe8\x5a\xaa\x5b\xda\xbf\x73\xe6\xcf\xaa\xc3\x61\x60\xf5\x5c\x4a\xa1\x6a\x29\x48\xaf\x55\x39\xf3\x3b\xdc\xc4\x36\xce\x2e\xe7\xa4\x58\x7a\xe5\x8c\x5e\xcf\x20\x52\x4c\x41\xdd\x6e\x23\xb4\xbd\x96\xcb\x95\x83\x49\x51\xfe\xb6\xaf\x00\x00\x00\xff\xff\x1f\x44\x12\x7d\x7e\x02\x00\x00")

func web_templates_index_html() ([]byte, error) {
	return bindata_read(
		_web_templates_index_html,
		"web/templates/index.html",
	)
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		return f()
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() ([]byte, error){
	"web/templates/index.html": web_templates_index_html,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for name := range node.Children {
		rv = append(rv, name)
	}
	return rv, nil
}

type _bintree_t struct {
	Func     func() ([]byte, error)
	Children map[string]*_bintree_t
}

var _bintree = &_bintree_t{nil, map[string]*_bintree_t{
	"web": &_bintree_t{nil, map[string]*_bintree_t{
		"templates": &_bintree_t{nil, map[string]*_bintree_t{
			"index.html": &_bintree_t{web_templates_index_html, map[string]*_bintree_t{}},
		}},
	}},
}}
