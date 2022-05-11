FROM docker.io/fedora:36

RUN dnf install -y --nodocs git python3-pip jq gettext helm make && dnf clean all && rm -rf /var/cache/yum
RUN pip3 install ansible google-auth google-oauth
RUN curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz > /tmp/google-cloud-sdk.tar.gz
RUN mkdir -p /usr/local/gcloud \
  && tar -C /usr/local/gcloud -xvf /tmp/google-cloud-sdk.tar.gz \
  && /usr/local/gcloud/google-cloud-sdk/install.sh
# Install kubectl
RUN curl -LO https://dl.k8s.io/release/v1.24.0/bin/linux/amd64/kubectl
RUN sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
# Copy in scaffolding project
RUN mkdir -p /scaffolding
RUN git clone http://github.com/jtaleric/scaffolding /scaffolding
RUN git clone https://github.com/cloud-bulldozer/benchmark-comparison && cd benchmark-comparison && sudo python3 setup.py develop && cd && rm -rf benchmark-comparison/
RUN pip3 cache purge
ENV PATH $PATH:/usr/local/gcloud/google-cloud-sdk/bin
