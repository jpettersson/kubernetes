FROM google/golang

RUN mkdir -p /gopath/src/github.com/GoogleCloudPlatform/
ADD . /gopath/src/github.com/GoogleCloudPlatform/kubernetes/
WORKDIR /gopath/src/github.com/GoogleCloudPlatform/kubernetes/cluster/addons/kube-dynamic-router
ENV GOPATH $GOPATH:/gopath/src/github.com/GoogleCloudPlatform/kubernetes/Godeps/_workspace
RUN make

VOLUME /gopath/src/github.com/GoogleCloudPlatform/kubernetes/cluster/addons/kube-dynamic-router

CMD ["sleep", "5"]