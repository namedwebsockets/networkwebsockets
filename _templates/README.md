Network Web Socket Proxy Template Files
===

This folder contains _source_ template files for Network Web Socket Proxy applications.

In order to use these templates in package builds they must be compiled. To compile the template files we use the [go-bindata](https://github.com/jteeuwen/go-bindata) tool.

`go-bindata` can be obtained using `go get`:

    go get github.com/jteeuwen/go-bindata/...

If any files in this folder change they need re-packing with the following command before re-running the Network Web Socket Proxy:

    cd `go list -f '{{.Dir}}' github.com/namedwebsockets/networkwebsockets`
    go-bindata -ignore=README\\.md -o templates.go _templates/

More information on `go-bindata` is available in the [go-bindata README](https://github.com/jteeuwen/go-bindata/blob/master/README.md).