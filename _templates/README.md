Internal Named WebSockets Template Files
===

This folder contains source template files for Named WebSockets Proxy applications.

In order to include these templates in package builds they must be compiled using the [go-bindata](https://github.com/jteeuwen/go-bindata) tool.

This tool can be obtained using `go get`:

    go get github.com/jteeuwen/go-bindata/...

If any files in this folder change they need re-packing with the following command before running the Named WebSockets Proxy as follows:

    cd `go list -f '{{.Dir}}' github.com/namedwebsockets/namedwebsockets`
    go-bindata -ignore=README\\.md -o templates.go _templates/

For more information, please consult the go-bindata [README](https://github.com/jteeuwen/go-bindata/blob/master/README.md).