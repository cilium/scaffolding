#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Dynamically build circleci configuration.

References:
* https://circleci.com/blog/building-cicd-pipelines-using-dynamic-config/
"""
import argparse
import json
import logging
import pprint
import typing as t
from pathlib import Path

import _jsonnet as jsonnet
import jinja2

logging.basicConfig(format="%(levelname)s:%(lineno)s:%(message)s", level=logging.DEBUG)

ROOT_PATH: Path = Path(__file__).resolve().parent
TEMPLATES_PATH: Path = ROOT_PATH / "templates"
MATRIX_OUTPUT_NAME: str = "matrix.json"
MATRIX_PATH: Path = ROOT_PATH / "matrix.jsonnet"
VARS_DIR_PATH: str = "vars"


def render_and_write_templates(output_path: Path, **kwargs) -> None:
    """
    Render templates in `TEMPLATES_PATH` and write result to disk.

    Parameters
    ----------
    output_path : Path
        Directory to output rendered templates to.
    kwargs
        Any kwargs are passed to templates as variables.
    """

    environment = jinja2.Environment(
        loader=jinja2.FileSystemLoader(TEMPLATES_PATH)
    )

    templates = environment.list_templates()
    logging.info("Rendering the following templates: %s", ", ".join(templates))
    logging.info(
        "Using the following context: %s", pprint.pformat(**kwargs, indent=4)
    )
    for template_name in templates:
        logging.debug("Rendering %s", template_name)
        template = environment.get_template(template_name)
        result = template.render(**kwargs)
        logging.debug(
            "Rendered %s as %s",
            template_name,
            result,
        )
        template_output_name = template_name.removesuffix(".j2")
        with open(
            output_path / template_output_name, "w", encoding="utf-8"
        ) as to_file_handler:
            logging.info(
                "Writing rendered template '%s' to '%s'",
                template_name,
                template_output_name,
            )
            to_file_handler.write(result)


def render_and_write_matrix(output_path: Path, **kwargs) -> t.Dict[str, str]:
    """
    Render testing matrix jsonnet file at `MATRIX_PATH`.

    Parameters
    ----------
    output_path : Path
        Path to write the rendered matrix to.
    kwargs
        Any kwargs given will be used as external variables for
        rendering
    
    Returns
    -------
    Scenarios pulled from testing matrix, as map from the scenario's
    name to its variables.
    """

    logging.info("Loading testing matrix")
    matrix_str = jsonnet.evaluate_file(
        str(MATRIX_PATH), ext_vars=kwargs
    )
    matrix = json.loads(matrix_str)
    logging.info(
        "Got the following testing matrix:\n%s", pprint.pformat(matrix)
    )

    matrix_out_path = output_path / MATRIX_OUTPUT_NAME
    with open(matrix_out_path, "w", encoding="utf-8") as matrix_file_handler:
        logging.info("Writing rendered testing matrix to '%s'", matrix_out_path)
        matrix_file_handler.write(matrix_str)

    scenarios = {scenario["_name"]: json.dumps(scenario) for scenario in matrix}
    for scenario, varfile in scenarios.items():
        varfile_output_path = output_path / VARS_DIR_PATH / (scenario + ".yml")
        with open(
            varfile_output_path, "w", encoding="utf-8"
        ) as varfile_file_handler:
            logging.info("Writing varfile '%s'", varfile_output_path)
            varfile_file_handler.write(varfile)
    
    return scenarios


def get_argument_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "output_dir",
        help="""Directory to output results to. Will create the following files/dirs:
        - matrix.json: Testing matrix
        - vars/: Ansible variable files passed to CircleCI workflow
        - pipeline.yml: CircleCI workflow
        """,
    )
    parser.add_argument(
        "--just-varfiles",
        help="If given, only varfiles will be written to output_dir",
        action=argparse.BooleanOptionalAction,
        default=False,
    )
    parser.add_argument(
        "--build-image",
        help="Build image in the workflow rather than pulling from quay",
        action=argparse.BooleanOptionalAction,
        default=False,
    )
    return parser


def main():
    parser = get_argument_parser()
    args = parser.parse_args()
    output_dir = Path(args.output_dir).resolve()
    # Create directories in case they don't exist
    (output_dir / VARS_DIR_PATH).mkdir(parents=True, exist_ok=True)

    scenarios = render_and_write_matrix(output_dir)

    if args.just_varfiles:
        return

    scenario_list = list(scenarios.keys())
    render_and_write_templates(
        output_dir,
        build_image=args.build_image,
        scenarios=scenario_list,
        vars_dir=VARS_DIR_PATH,
    )


if __name__ == "__main__":
    main()
