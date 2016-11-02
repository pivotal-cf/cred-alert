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

var _web_templates_index_html = []byte("\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\x8c\x94\xcd\x8e\xdb\x20\x10\xc7\xcf\xe4\x29\xa6\xf4\xbc\x66\xa5\x95\x7a\xd8\x62\x0e\x4d\x7b\xed\x56\x55\x2f\x3d\x12\x33\x09\xa8\x04\x2c\x20\x2b\xb9\x56\xde\xbd\xc2\x76\x30\xeb\x7e\x6c\x4f\x1e\x86\x19\xe6\xcf\xfc\x06\xf3\x37\x1f\x9f\xf6\xdf\xbe\x7f\xf9\x04\x3a\x9d\xad\xd8\xf1\xfc\x01\x2b\xdd\xa9\xa5\xe8\xa8\xd8\x01\x70\x8d\x52\x65\x03\x80\x27\x93\x2c\x8a\x7d\x40\x85\x2e\x19\x69\x61\xef\x2f\x2e\xc1\x61\x80\xa7\x70\x92\xce\xfc\x94\xc9\x78\xc7\xd9\x1c\x37\xe7\x58\xe3\x7e\x40\x40\xdb\xd2\x98\x06\x8b\x51\x23\x26\x0a\x3a\xe0\xb1\xa5\x3a\xa5\xfe\x91\xb1\xe1\x62\x9a\x41\x6a\xef\x65\x6f\x62\xd3\xf9\x33\xeb\x2f\x01\xd9\x7d\xf3\xae\xb9\x9f\xcc\xbb\xb3\x71\x4d\x17\xe3\x2c\x88\xdd\x14\xf1\x83\x57\xc3\x52\x46\x99\x67\x30\xaa\xa5\x9d\x77\x09\x5d\xa2\xb3\x3b\xcb\x7f\xf8\x4d\x70\x84\x0f\x5b\xc5\xfa\xa1\x24\x24\x79\xb0\x08\x9d\x95\x31\xb6\x74\xaa\x3e\x7b\x56\xf3\xee\xe0\x83\xc2\x80\xaa\x54\xc9\x69\x6b\x9f\x8a\x47\x6c\xda\xa2\xb7\xfb\x5b\x65\x2f\x63\xf2\x6a\x3a\xb4\x78\xc6\x31\x48\x77\x42\x68\xea\x83\xe3\xf5\x5a\xc9\x08\x2f\x6b\x28\xc1\xe5\xd2\xed\xb7\xe3\xd8\x7c\x96\x67\xbc\x5e\xa9\x28\x26\x67\x52\x70\x96\xd4\x36\x6b\x1c\x9b\x55\xdc\xa4\x2d\xc7\xd6\x71\x9c\xd5\xb5\xc6\x11\x9d\x2a\x42\x38\x9b\x1a\x55\x94\xf3\x43\x00\x26\xfe\xb8\xd0\x61\x8d\xfa\x1b\xab\xaf\xd8\xfb\x68\x92\x0f\x43\x4d\xea\x95\x66\x4c\x23\x31\xcd\x5c\x4b\x8f\xd6\xcb\xf4\x68\xf1\x98\xde\x53\xb1\x23\x0b\xe3\x3c\x2f\x55\x53\xfe\x9b\x39\x01\xe0\x9d\xec\x73\xb9\xba\x91\x37\xd7\xb4\xbf\x90\x23\x37\xd2\xf5\x15\x32\x63\xf2\xef\x11\x20\x35\x7d\x52\xdd\xb5\x9c\x63\x30\x5f\x95\x2c\xc8\x49\xc5\x6d\x0b\x96\xbc\xce\x94\xdc\x70\x92\x95\x24\x29\x10\x49\x0d\x97\x33\x65\x9e\xe7\x77\x38\x3f\x3f\xce\xe6\x7f\xc7\xaf\x00\x00\x00\xff\xff\x3a\x77\x20\x03\x4c\x04\x00\x00")

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
	Func func() ([]byte, error)
	Children map[string]*_bintree_t
}
var _bintree = &_bintree_t{nil, map[string]*_bintree_t{
	"web": &_bintree_t{nil, map[string]*_bintree_t{
		"templates": &_bintree_t{nil, map[string]*_bintree_t{
			"index.html": &_bintree_t{web_templates_index_html, map[string]*_bintree_t{
			}},
		}},
	}},
}}
