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

var _web_templates_index_html = []byte("\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\x8c\x94\xc1\x6e\xdc\x20\x10\x86\xcf\xec\x53\x4c\xe9\x39\x26\x52\xa4\x1e\x52\xec\x43\xb7\xbd\x36\x55\xd5\x4b\x8f\xd8\xcc\x2e\xa8\x2c\x58\xc0\x46\x72\xad\x7d\xf7\x0a\xdb\x8b\x89\xd3\x76\x73\xf2\x30\xcc\x30\x3f\xf3\x0d\xe6\xef\x3e\x3f\xed\x7f\xfc\xfc\xf6\x05\x54\x3c\x99\x66\xc7\xd3\x07\x8c\xb0\xc7\x9a\xa2\xa5\xcd\x0e\x80\x2b\x14\x32\x19\x00\x3c\xea\x68\xb0\xd9\x7b\x94\x68\xa3\x16\x06\xf6\xee\x6c\x23\xb4\x03\x3c\xf9\xa3\xb0\xfa\xb7\x88\xda\x59\xce\xe6\xb8\x39\xc7\x68\xfb\x0b\x3c\x9a\x9a\x86\x38\x18\x0c\x0a\x31\x52\x50\x1e\x0f\x35\x55\x31\xf6\x8f\x8c\x0d\x67\x5d\x0d\x42\x39\x27\x7a\x1d\xaa\xce\x9d\x58\x7f\xf6\xc8\xee\xab\x0f\xd5\xfd\x64\xde\x9d\xb4\xad\xba\x10\x66\x41\xec\xaa\x88\xb7\x4e\x0e\x4b\x19\xa9\x9f\x41\xcb\x9a\x76\xce\x46\xb4\x91\xce\xee\x24\xff\xe1\x95\xe0\x00\x9f\xb6\x8a\xd5\x43\x4e\x88\xa2\x35\x08\x9d\x11\x21\xd4\x74\xaa\x3e\x7b\x56\xf3\xae\x75\x5e\xa2\x47\x99\xab\xa4\xb4\xb5\x4f\xd9\xd3\x6c\xda\xa2\xb6\xfb\x5b\x65\x2f\x63\xd2\x6a\x3a\x34\x7b\xc6\xd1\x0b\x7b\x44\xa8\xca\x83\xc3\xe5\x52\xc8\xf0\x2f\x6b\xc8\x86\x8b\xa5\xdb\xef\xc7\xb1\xfa\x2a\x4e\x78\xb9\xd0\x26\x9b\x9c\x89\x86\xb3\x28\xb7\x59\xe3\x58\xad\xe2\x26\x6d\x29\xb6\x8c\xe3\xac\xac\x35\x8e\x68\x65\x16\xc2\xd9\xd4\xa8\xac\x9c\xb7\x1e\x58\xf3\xd7\x85\xf2\x6b\xd4\xbf\x58\x7d\xc7\xde\x05\x1d\x9d\x1f\x4a\x52\x37\x9a\x31\x8d\xc4\x34\x73\x35\x3d\x18\x27\xe2\xa3\xc1\x43\xfc\x48\x9b\x1d\x59\x18\xa7\x79\x29\x9a\xf2\x66\xe6\x04\x80\x77\xa2\x4f\xe5\xca\x46\x5e\x5d\xd3\xfe\x42\x8e\x5c\x49\x97\x57\x48\x8c\xc9\xff\x47\x80\x94\xf4\x49\x71\xd7\x7c\x8e\xc6\x74\x55\xb2\x20\x27\x05\xb7\x2d\x58\x72\x9b\x29\xb9\xe2\x24\x2b\x49\x92\x21\xbe\x26\xcc\x99\xd4\xcf\xf3\x63\x9c\xdf\x20\x67\xf3\x0f\xe4\x4f\x00\x00\x00\xff\xff\x6e\x76\x80\x7f\x51\x04\x00\x00")

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
