FROM docker.io/fedora:36

RUN dnf install -y --nodocs git python3-pip && dnf clean all && rm -rf /var/cache/yum
RUN pip install dash elasticsearch

RUN mkdir -p /scaffolding
RUN git clone http://github.com/jtaleric/scaffolding /scaffolding
RUN git clone https://github.com/cloud-bulldozer/benchmark-comparison && cd benchmark-comparison && sudo python3 setup.py develop && cd && rm -rf benchmark-comparison/
RUN pip3 cache purge
