// Create testing matrix for CI pipeline
//
// Each entry in the outputted array is JSON-formatted vars file that can be
// passed to ansible.
//
// Parameters universal to all items in the matrix
local UNI_PARAMS = {
    'num_nodes': 2,
    'cluster_prefix': 'circleci-cilium-perf-ci',
    'gke': {
        'sa_file': '/tmp/workspace/sa.json'
    },
    'start_time': '1653585502'
};
local create_matrix_item(cilium_version, kernel) = {
    'cilium_version': cilium_version,
    'kernel': kernel,
    '_name': 'CV=' + cilium_version + '-K=' + kernel,
} + UNI_PARAMS;

local cilium_versions = [
    "v1.12.0-rc2",
    "v1.11-5",
    "v1.10-11"
];
local kernel_versions = [
    "5.4",
    "5.16",
];
local extras = [
    create_matrix_item(
        "v1.12.0-rc2",
        "5.18"
    )
];

local matrix = [
    create_matrix_item(
        cilium_version, 
        kernel_version
    ) 
    for cilium_version in cilium_versions
    for kernel_version in kernel_versions
];

// matrix + extras
[ create_matrix_item("v1.12.0-rc2", "5.18") ]