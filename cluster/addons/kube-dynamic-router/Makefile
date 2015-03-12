all: kube-dynamic-router

kube-dynamic-router: kube-dynamic-router.go
	CGO_ENABLED=0 go build -a -installsuffix cgo --ldflags '-w' ./kube-dynamic-router.go

clean:
	rm -f kube-dynamic-router
