FROM ns-base:latest
RUN apt install -y hwloc libhwloc-dev
COPY worker /usr/local/bin/
ENTRYPOINT [ "worker" ]