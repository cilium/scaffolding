#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Create kubeconfig from output of `google.cloud.gcp_container_cluster`

Use `google-auth` module to get an OAuth bearer token for the given
service account, which can then be passed to kubectl in order to
authenticate to the cluster.

The token does expire, however re-running this script will refresh
the token when needed.

References:
* https://github.com/mie00/gke-kubeconfig
* https://google-auth.readthedocs.io/en/master/user-guide.html

Requires `google-auth` and `requests` to be installed.
"""
import argparse
import json
import logging
import os
import shlex
import subprocess
import sys
import typing as t

from google.auth.transport.requests import Request
from google.oauth2 import service_account

KUBECONFIG_TEMPLATE = """
apiVersion: v1
kind: Config
clusters:
- name: {cluster_name}
  cluster:
    server: https://{cluster_server}
    certificate-authority-data: {cluster_ca}
users:
- name: my-gke-sa-user
  user:
    token: {user_token}
contexts:
- context:
    cluster: {cluster_name}
    user: my-gke-sa-user
  name: {cluster_name}
current-context: {cluster_name}
"""
GOOGLE_AUTH_API_BASE = "https://www.googleapis.com/auth/"


def get_google_sa_token(path_to_sa: str) -> str:
    """Use given service account private key to get token for k8s api server."""

    credentials = service_account.Credentials.from_service_account_file(
        path_to_sa,
        scopes=[
            GOOGLE_AUTH_API_BASE + scope
            for scope in ("userinfo.email", "cloud-platform")
        ],
    )
    credentials.refresh(Request())
    return credentials.token


def build_kubeconfig(
    kubeconfig_template: str,
    cluster_server: str,
    cluster_name: str,
    cluster_ca: str,
    user_token: str,
) -> str:
    """Use the given arguments to build a kubeconfig."""

    return kubeconfig_template.format(
        cluster_name=cluster_name,
        cluster_server=cluster_server,
        cluster_ca=cluster_ca,
        user_token=user_token,
    )


def get_kubeconfig_params_from_gcp_container_cluster_json(
    gcp_container_cluster_json: str,
) -> t.Dict[str, str]:
    """Get params for `build_kubeconfig` from `gcp_container_cluster` results."""

    results = json.loads(gcp_container_cluster_json)
    return {
        "cluster_ca": results["masterAuth"]["clusterCaCertificate"],
        "cluster_name": results["name"],
        "cluster_server": results["endpoint"],
    }


def run_kubectl_command(cmd: str) -> None:
    """Run the given kubectl command, assuming `set_kubeconfig`."""

    cmd = "kubectl " + cmd
    logging.info("Running '%s'", cmd)
    try:
        result = subprocess.run(
            shlex.split(cmd),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            env=os.environ,
            timeout=20,
            check=True,
        )
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as err:
        logging.warning(
            "Got error while running kubectl command '%s': %s, %s",
            cmd,
            err,
            err.stdout.decode("utf-8"),
        )
        raise

    logging.info("Result: %s", result.stdout.decode("utf-8"))


def setup_logging() -> None:
    """Configure global console logging."""

    logging.basicConfig(
        format="%(asctime)s - %(levelname)s: %(message)s",
        level=logging.INFO,
        stream=sys.stdout,
    )


def create_argument_parser() -> argparse.ArgumentParser:
    """Create argument parser for the script."""

    parser = argparse.ArgumentParser()
    parser.add_argument(
        "gcp_ansible_json",
        help="File path to output of google.cloud.gcp_container_cluster",
    )
    parser.add_argument(
        "service_account_file", help="Path to private key of Google IAM service account"
    )
    parser.add_argument("kubeconfig_dest", help="Destination of kubeconfig")

    return parser


def main() -> None:
    setup_logging()
    parser = create_argument_parser()
    args = parser.parse_args()

    logging.info(
        "Parsing through gcp_container_cluster json: %s",
        args.gcp_ansible_json,
    )
    with open(args.gcp_ansible_json, "r") as gcp_ansible_json_file_handler:
        gcp_ansible_json = gcp_ansible_json_file_handler.read()
    kubeconfig_args = get_kubeconfig_params_from_gcp_container_cluster_json(
        gcp_ansible_json
    )

    logging.info("Getting token for sa authentication")
    user_token = get_google_sa_token(args.service_account_file)
    kubeconfig_args["user_token"] = user_token

    kubeconfig = build_kubeconfig(KUBECONFIG_TEMPLATE, **kubeconfig_args)
    with open(args.kubeconfig_dest, "w") as kubeconfig_file_handler:
        kubeconfig_file_handler.write(kubeconfig)
    logging.info("Kubeconfig written to %s", args.kubeconfig_dest)

    os.environ["KUBECONFIG"] = args.kubeconfig_dest
    run_kubectl_command("config view")
    run_kubectl_command("get nodes")


if __name__ == "__main__":
    main()
