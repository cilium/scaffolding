FROM docker.io/fedora:36

RUN dnf install -y --nodocs \
    git \
    helm \
    jq \
    make \
    python3-pip && \
  dnf clean all && \
  pip3 install --no-cache-dir \
    ansible-core \
    google-auth \
    requests
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
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
