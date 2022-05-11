FROM docker.io/fedora:36

RUN dnf install -y --nodocs git python3-pip jq && dnf clean all
RUN pip3 install --upgrade pip
RUN pip3 install ansible
RUN pip3 install google-auth
RUN pip3 install google-oauth
RUN mkdir -p /scaffolding
RUN git clone http://github.com/jtaleric/scaffolding /scaffolding
RUN curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz > /tmp/google-cloud-sdk.tar.gz
RUN mkdir -p /usr/local/gcloud \
  && tar -C /usr/local/gcloud -xvf /tmp/google-cloud-sdk.tar.gz \
  && /usr/local/gcloud/google-cloud-sdk/install.sh
ENV PATH $PATH:/usr/local/gcloud/google-cloud-sdk/bin
