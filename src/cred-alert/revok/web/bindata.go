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

var _web_templates_index_html = []byte("\x1f\x8b\x08\x00\x00\x09\x6e\x88\x00\xff\xb4\x53\xc1\x4e\xc3\x30\x0c\x3d\xb3\xaf\x30\xe1\xbe\xfc\x80\x9b\xcb\xe0\xca\x10\xe2\xc2\xd1\x5d\xcc\x1a\x29\x4b\xa6\x34\x4c\x1a\x51\xff\x1d\xa5\xdd\xb2\xac\x20\xed\x02\xa7\xc6\xf6\xf3\xf3\x73\x5f\x82\xf7\x8f\xeb\xd5\xdb\xfb\xcb\x13\x74\x71\x67\xd5\x02\xf3\x07\x2c\xb9\x6d\x23\xd8\x09\xb5\x00\xc0\x8e\x49\xe7\x03\x00\x46\x13\x2d\xab\x55\x60\xcd\x2e\x1a\xb2\xb0\xf2\x9f\x2e\x42\x7b\x84\x75\xd8\x92\x33\x5f\x14\x8d\x77\x28\x27\x5c\x6e\x96\xe7\x6e\x6c\xbd\x3e\x9e\x68\xb4\x39\x80\xd1\x8d\xd8\x78\x17\xd9\x45\x31\xa5\x33\x3f\xb5\x96\x61\x63\xa9\xef\x1b\x31\x06\xa5\x96\xab\x17\x25\x25\xa3\x66\x83\xbb\x79\x7d\x2e\xf6\x1a\x93\xa3\x91\xb4\x64\x52\x0a\xe4\xb6\x0c\xcb\x9a\xb8\x1f\x86\x4a\x46\xb8\x9e\xa1\x15\x12\x74\x81\x3f\x1a\xf1\x90\xd2\xf2\x99\x76\x3c\x0c\x42\x95\x23\x4a\x52\x28\xa3\x9e\x77\xa5\xb4\xbc\x88\x1b\xb5\x65\x6c\x8d\x43\x59\xcf\x4a\x89\x9d\x2e\x42\x50\x8e\xff\xa7\x28\xc7\x36\x80\x54\xbf\x06\x5d\x28\xa8\x1b\xdb\x9d\x0c\xc8\xde\x54\xab\xcc\x0d\xb9\xc3\x0d\xed\x73\x63\xbd\xe3\x39\x75\xc3\xae\x57\xde\xfb\xde\x44\x1f\x8e\x7f\x6a\x56\xa1\x35\x7c\xc3\xab\x7f\xb3\x65\xf1\xd3\x25\x94\xda\x1c\xa6\x67\x30\xdd\x7e\x94\xd3\x33\xfb\x0e\x00\x00\xff\xff\x99\x7c\x11\xe1\x77\x03\x00\x00")

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
