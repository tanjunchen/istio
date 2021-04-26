# 编译打包 istio proxy 1.6.14 打包文档记录

在 /root/go/src/istio.io 上克隆 istio 镜像仓库

git clone https://github.com/istio/istio.git && cd istio

切换到 1.6.14 分支

git checkout -b 1.6.14 1.6.14

备注：尽量最好请先自行解决网络问题，解决翻墙问题。

```
docker login --username=tanjunchen20 registry.cn-hangzhou.aliyuncs.com

docker pull registry.cn-hangzhou.aliyuncs.com/tanjunchen/build-tools:release-1.6-2020-11-13T15-30-50

需要翻墙下载以下镜像

gcr.io/istio-testing/build-tools:release-1.6-2020-11-13T15-30-50
```

执行 make VERSION=1.6.14 rpm/builder-image 命令

如果编译镜像出现以下错误，目前是注释了 tools/packaging/rpm/Dockerfile.build 中的 /usr/bin/ninja-build /usr/bin/ninja 软链接操作即可。

Step 5/11 : RUN curl -o /usr/local/bin/bazel -L https://github.com/bazelbuild/bazelisk/releases/download/v1.1.0/bazelisk-linux-amd64 &&     chmod +x /usr/local/bin/bazel
 ---> Using cache
 ---> e59b3bc8b8ca
Step 6/11 : RUN ln -s /usr/bin/cmake3 /usr/bin/cmake &&     ln -s /usr/bin/ninja-build /usr/bin/ninja
 ---> Running in c46f7d52d3f5
ln: failed to create symbolic link '/usr/bin/ninja': File exists
The command '/bin/sh -c ln -s /usr/bin/cmake3 /usr/bin/cmake &&     ln -s /usr/bin/ninja-build /usr/bin/ninja' returned a non-zero code: 1
tools/packaging/rpm/rpm.mk:32: recipe for target 'rpm/builder-image' failed
make[1]: *** [rpm/builder-image] Error 1
make: *** [Makefile:46: rpm/builder-image] Error 2

编译镜像成功，如下所示：

istio-rpm-builder:latest  3.45GB

```
[root@mesh-10-20-11-190 istio]# make VERSION=1.6.14 rpm/builder-image
docker build -t istio-rpm-builder -f /work/tools/packaging/rpm/Dockerfile.build /work/tools/packaging/rpm
Sending build context to Docker daemon  27.14kB
Step 1/11 : FROM centos:7
 ---> 8652b9f0cb4c
Step 2/11 : RUN yum install -y centos-release-scl epel-release &&     yum update -y &&     yum install -y fedpkg sudo devtoolset-7-gcc devtoolset-7-gcc-c++                    devtoolset-7-binutils java-1.8.0-openjdk-headless rsync                    rh-git218 wget unzip which make cmake3 patch ninja-build                    devtoolset-7-libatomic-devel openssl python27 libtool autoconf &&     yum clean all
 ---> Using cache
 ---> 8fb1d6dd0c17
Step 3/11 : RUN curl -o /root/go.tar.gz https://dl.google.com/go/go1.13.4.linux-amd64.tar.gz &&     tar zxf /root/go.tar.gz -C /usr/local
 ---> Using cache
 ---> a27dac7c7d03
Step 4/11 : ENV GOROOT=/usr/local/go     PATH=/usr/local/go/bin:/opt/rh/rh-git218/root/usr/bin:/opt/rh/devtoolset-7/root/usr/bin:/opt/llvm/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH}
 ---> Using cache
 ---> 34db8194bcaa
Step 5/11 : RUN curl -o /usr/local/bin/bazel -L https://github.com/bazelbuild/bazelisk/releases/download/v1.1.0/bazelisk-linux-amd64 &&     chmod +x /usr/local/bin/bazel
 ---> Using cache
 ---> e59b3bc8b8ca
Step 6/11 : RUN ln -s /usr/bin/cmake3 /usr/bin/cmake
 ---> Using cache
 ---> 88a0535d8d22
Step 7/11 : RUN echo "/opt/rh/httpd24/root/usr/lib64" > /etc/ld.so.conf.d/httpd24.conf &&     ldconfig
 ---> Using cache
 ---> 05ec9f46546b
Step 8/11 : ENV LLVM_VERSION=9.0.0
 ---> Using cache
 ---> 4b2748bd32c5
Step 9/11 : ENV LLVM_DISTRO="x86_64-linux-sles11.3"
 ---> Using cache
 ---> ab821fe0a7d6
Step 10/11 : ENV LLVM_RELEASE="clang+llvm-${LLVM_VERSION}-${LLVM_DISTRO}"
 ---> Using cache
 ---> 8b092c276236
Step 11/11 : RUN curl -fsSL --output ${LLVM_RELEASE}.tar.xz https://releases.llvm.org/${LLVM_VERSION}/${LLVM_RELEASE}.tar.xz &&     tar Jxf ${LLVM_RELEASE}.tar.xz &&     mv ./${LLVM_RELEASE} /opt/llvm &&     chown -R root:root /opt/llvm &&     rm ./${LLVM_RELEASE}.tar.xz &&     echo "/opt/llvm/lib" > /etc/ld.so.conf.d/llvm.conf &&     ldconfig
 ---> Running in 266cf6894400
/usr/bin/ninja-build /usr/bin/ninjaRemoving intermediate container 266cf6894400
 ---> 5a26ceb0e73d
Successfully built 5a26ceb0e73d
Successfully tagged istio-rpm-builder:latest
```

执行命令：make VERSION=1.6.14 rpm/proxy

执行 make VERSION=1.6.14 rpm/proxy 命令的过程中，如果出现以下错误：

```
docker run --rm -it \
        -v /go:/go \
			-w /builder \
        -e USER= \
			-e ISTIO_ENVOY_VERSION=1ef6cb53abbb057185f4bcb60e28cf92c3a174ad \
			-e ISTIO_GO=/work \
			-e ISTIO_OUT=/work/out/linux_amd64 \
			-e PACKAGE_VERSION=1.6.14 \
			-e USER_ID=0 \
			-e GROUP_ID=994 \
			istio-rpm-builder \
			/work/tools/packaging/rpm/build-proxy-rpm.sh
docker: Error response from daemon: OCI runtime create failed: container_linux.go:367: starting container process caused: exec: "/work/tools/packaging/rpm/build-proxy-rpm.sh": stat /work/tools/packaging/rpm/build-proxy-rpm.sh: no such file or directory: unknown.
ERRO[0003] error waiting for container: context canceled 
tools/packaging/rpm/rpm.mk:18: recipe for target 'rpm/proxy' failed
```

可以更改 tools/packaging/rpm/rpm.mk 下的 

将 -v /root/go/src/istio.io/istio:/work 挂载在容器中即可。

```
rpm/proxy:
	docker run --rm -it \
        -v ${GO_TOP}:${GO_TOP} \
        -v /root/go/src/istio.io/istio:/work \
				-w /builder \
        -e USER=${USER} \
				-e ISTIO_ENVOY_VERSION=${ISTIO_ENVOY_VERSION} \
				-e ISTIO_GO=${ISTIO_GO} \
				-e ISTIO_OUT=${ISTIO_OUT} \
				-e PACKAGE_VERSION=${PACKAGE_VERSION} \
				-e USER_ID=$(shell id -u) \
				-e GROUP_ID=$(shell id -g) \
				istio-rpm-builder \
				${PWD}/tools/packaging/rpm/build-proxy-rpm.sh
```

如果 istio/proxy 仓库克隆不下来，则可以执行以下操作。

```
git clone  https://github.com/istio/proxy.git istio-proxy
#git clone  https://github.com.cnpmjs.org/istio/proxy.git istio-proxy
```