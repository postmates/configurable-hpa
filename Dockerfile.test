FROM golang:1.11.5-alpine3.8

# Variables for Kubebuilder
ENV version=1.0.4
ENV arch=amd64
ENV checksum=3cc1b63202a786bf8418b7ed0c8167566dfc5e2f4a780dd9fb88e04580f6cdd2
env PATH=$PATH:/usr/local/kubebuilder/bin

## add edge packets for installing etcd from @testing
## after installing kubebuilder - copy etcd binary to the installed folder
## the kubebuilder-one doesn't work properly
## and remove unneeded files
RUN    echo '@testing http://dl-cdn.alpinelinux.org/alpine/edge/testing' >> /etc/apk/repositories \
    && echo '@edge    http://dl-cdn.alpinelinux.org/alpine/edge/main'    >> /etc/apk/repositories \
    && apk add --no-cache --upgrade apk-tools@edge \
    && apk add make gcc musl-dev ssl_client etcd@testing \
    && wget https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/kubebuilder_${version}_linux_${arch}.tar.gz \
    && echo "$checksum  kubebuilder_1.0.4_linux_amd64.tar.gz" | sha256sum -c - \
    && tar -zxvf kubebuilder_${version}_linux_${arch}.tar.gz \
    && mv kubebuilder_${version}_linux_${arch} /usr/local/kubebuilder \
    && rm kubebuilder_${version}_linux_${arch}.tar.gz \
    && cp /usr/bin/etcd /usr/local/kubebuilder/bin/etcd \
    && rm /usr/local/kubebuilder/bin/client-gen \
    && rm /usr/local/kubebuilder/bin/conversion-gen \
    && rm /usr/local/kubebuilder/bin/deepcopy-gen \
    && rm /usr/local/kubebuilder/bin/defaulter-gen \
    && rm /usr/local/kubebuilder/bin/gen-apidocs \
    && rm /usr/local/kubebuilder/bin/informer-gen \
    && rm /usr/local/kubebuilder/bin/kube-controller-manager \
    && rm /usr/local/kubebuilder/bin/kubebuilder \
    && rm /usr/local/kubebuilder/bin/kubebuilder-gen \
    && rm /usr/local/kubebuilder/bin/lister-gen \
    && rm /usr/local/kubebuilder/bin/openapi-gen \
    && rm /usr/local/kubebuilder/bin/vendor.tar.gz
