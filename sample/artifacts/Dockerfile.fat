# new image
FROM scratch
# Imported from -
ADD file:62400a49cced0d7521560b501f6c52227c60f5e2fecd0fef20e4d0e1558f7301 in /
RUN echo '#!/bin/sh' > /usr/sbin/policy-rc.d && \
	echo 'exit 101' >> /usr/sbin/policy-rc.d && \
	chmod +x /usr/sbin/policy-rc.d && \
	dpkg-divert --local --rename --add /sbin/initctl && \
	cp -a /usr/sbin/policy-rc.d /sbin/initctl && \
	sed -i 's/^exit.*/exit 0/' /sbin/initctl && \
	echo 'force-unsafe-io' > /etc/dpkg/dpkg.cfg.d/docker-apt-speedup && \
	echo 'DPkg::Post-Invoke { "rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true"; };' > /etc/apt/apt.conf.d/docker-clean && \
	echo 'APT::Update::Post-Invoke { "rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true"; };' >> /etc/apt/apt.conf.d/docker-clean && \
	echo 'Dir::Cache::pkgcache ""; Dir::Cache::srcpkgcache "";' >> /etc/apt/apt.conf.d/docker-clean && \
	echo 'Acquire::Languages "none";' > /etc/apt/apt.conf.d/docker-no-languages && \
	echo 'Acquire::GzipIndexes "true"; Acquire::CompressionTypes::Order:: "gz";' > /etc/apt/apt.conf.d/docker-gzip-indexes
RUN sed -i 's/^#\s*\(deb.*universe\)$/\1/g' /etc/apt/sources.list
CMD [/bin/bash]
# end of image: ubuntu (id: 5ba9dab47459d81c0037ca3836a368a4f8ce5050505ce89720e1fb8839ea048a tags: 14.04.1,latest,trusty,14.04)

# new image
RUN apt-get update && \
	apt-get install -y curl software-properties-common python-software-properties && \
	add-apt-repository ppa:chris-lea/node.js && \
	apt-get update && \
	apt-get install -y build-essential 		nodejs && \
	mkdir -p /opt/my/service
COPY dir:d23e668df9774f0bb80d5cf05fa913ba58fb912ad9df85e3bed6deebc2d8046a in /opt/my/service
WORKDIR /opt/my/service
RUN npm install
EXPOSE 8000/tcp
ENTRYPOINT ["node" "/opt/my/service/server.js"]
# end of image: my/sample-node-app (id: 0c53ebce746743632efc8886ab93d024d7ea1e42cbc7be5630aee237f16c30ac tags: latest)
