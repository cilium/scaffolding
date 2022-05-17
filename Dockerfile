FROM docker.io/fedora:36

RUN dnf install -y --nodocs \
    helm \
    jq \
    python3-pip && \
  dnf clean all && \
  pip3 install --no-cache-dir \
    ansible-core \
    google-auth \
    requests
RUN curl -LO https://dl.k8s.io/release/v1.24.0/bin/linux/amd64/kubectl && \
  install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
RUN curl -sSL https://api.github.com/repos/cloud-bulldozer/benchmark-comparison/tarball | tar -xzf - -C /opt && \
  export bc_dir=/opt/$(ls /opt | grep cloud-bulldozer-benchmark-comparison) && \
  pip install $bc_dir && \
  rm -rf $bc_dir
RUN ansible-galaxy collection install \
    community.general \
    google.cloud

COPY . /scaffolding
