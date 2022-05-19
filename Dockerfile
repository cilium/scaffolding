FROM docker.io/fedora:36

RUN mkdir -p /usr/local/gcloud && \
  curl -sSL https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz | tar -xzf - -C /usr/local/gcloud && \
  /usr/local/gcloud/google-cloud-sdk/install.sh \
    --override-components='core' \
    --override-components='kubectl' && \
  rm -rf /usr/local/gcloud/google-cloud-sdk/.install/backup
ENV PATH="/usr/local/gcloud/google-cloud-sdk/bin:${PATH}"
RUN dnf install -y --nodocs \
    helm \
    python3-pip && \
  dnf clean all && \
  pip3 install --no-cache-dir \
    ansible-core \
    google-auth \
    requests
RUN curl -sSL https://api.github.com/repos/cloud-bulldozer/benchmark-comparison/tarball | tar -xzf - -C /opt && \
  export bc_dir=/opt/$(ls /opt | grep cloud-bulldozer-benchmark-comparison) && \
  pip install $bc_dir && \
  rm -rf $bc_dir
RUN mkdir -p ~/.ansible/collections/ansible_collections/google/cloud && \
  curl -sSL https://galaxy.ansible.com/download/google-cloud-1.0.2.tar.gz | tar -xzf - -C ~/.ansible/collections/ansible_collections/google/cloud && \
  mkdir -p  ~/.ansible/collections/ansible_collections/community/general && \
  curl -sSL https://galaxy.ansible.com/download/community-general-5.0.0.tar.gz | tar -xzf - -C ~/.ansible/collections/ansible_collections/community/general

WORKDIR /scaffolding
COPY . /scaffolding
