# egw-masq-delay

Run a masquerade delay test for Cilium's EGW feature, using the
components in the `egw-scale-utils` directory in the repository's
root.

## Usage


```
$ ./setup-cl2.sh
$ ./setup-kind.sh
$ ./run-kind.sh 15 3 baseline
$ ./run-kind.sh 15 3
```
