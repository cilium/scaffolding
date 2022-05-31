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
import yaml

logging.basicConfig(format="%(levelname)s:%(lineno)s:%(message)s", level=logging.DEBUG)

ROOT_PATH: Path = Path(__file__).resolve().parent
TEMPLATES_PATH: Path = ROOT_PATH / "templates"
MATRIX_OUTPUT_NAME: str = "matrix.json"
MATRIX_PATH: Path = ROOT_PATH / "matrix.jsonnet"
VARS_DIR_PATH: str = "vars"


class TemplateHandler:
    """Handle rendering of jinja2 templates"""

    def __init__(self):
        self.loader = jinja2.FileSystemLoader(TEMPLATES_PATH)
        self.environment = jinja2.Environment(
            loader=self.loader,
        )
        self.context: t.Dict[str, str] = {}
        self.rendered: t.Dict[str, str] = {}

    def add_vars(self, **kwargs):
        """Add variables into the context for jinja2 evaluation."""
        self.context.update(**kwargs)

    def render_all(self):
        """
        Render all templates in `TEMPLATES_PATH`.

        Results will be written to `output_path`, using the same
        filename as the templates but with the `.j2` extension removed.
        For instance, if a template named `mytemplate.yml.j2` is
        rendered, it will be placed into the `output_path` as
        `mytemplate.yml`
        """
        templates = self.environment.list_templates()
        logging.info("Rendering the following templates: %s", ", ".join(templates))
        logging.info(
            "Using the following context: %s", pprint.pformat(self.context, indent=4)
        )
        for template_name in templates:
            logging.debug("Rendering %s", template_name)
            template = self.environment.get_template(template_name)
            result = template.render(**self.context)
            logging.debug(
                "Rendered %s as %s",
                template_name,
                result,
            )
            self.rendered[template_name] = result

    def write_all(self, output_path: Path):
        """
        Write all rendered templates to the given `output_path`.

        Results will be written to `output_path`, using the same
        filename as the templates but with the `.j2` extension removed.
        For instance, if a template named `mytemplate.yml.j2` is
        rendered, it will be placed into the `output_path` as
        `mytemplate.yml`
        """

        for template_name, result in self.rendered.items():
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


class MatrixHandler:
    """Handle loading testing matrix jsonnet file"""

    def __init__(self):
        self.matrix: t.List[t.Dict[str, str]] = []
        self.matrix_str: t.Optional[str] = None
        self.loaded: bool = False
        self.ext_vars: t.Dict[str, str] = {}
        self.scenarios: t.Dict[str, str] = {}

    def add_vars(self, **kwargs):
        """Add 'external variables' to the jsonnet evaluation."""
        self.ext_vars.update(**kwargs)

    def load(self):
        """
        Read and evaluate testing matrix jsonnet file.

        File will be pulled from `MATRIX_PATH`.
        """
        logging.info("Loading testing matrix")
        self.matrix_str = jsonnet.evaluate_file(
            str(MATRIX_PATH), ext_vars=self.ext_vars
        )
        self.matrix = json.loads(self.matrix_str)
        logging.info(
            "Got the following testing matrix:\n%s", pprint.pformat(self.matrix)
        )
        for scenario in self.matrix:
            logging.debug("Converting scenario to yaml: %s", scenario)
            varfile = yaml.dump(scenario)
            name = scenario["_name"]
            logging.debug("Resulting yaml: %s", varfile)
            self.scenarios[name] = varfile
        self.loaded = True

    def write(self, output_path: Path):
        """
        Write evaluated matrix to the given output directory.

        Resulting file will be named `MATRIX_OUTPUT_NAME`.
        """
        if not self.loaded:
            self.load()
        if self.matrix_str is None:
            raise ValueError(
                "Unable to write testing matrix as it hasn't been loaded yet"
            )
        matrix_out_path = output_path / MATRIX_OUTPUT_NAME
        with open(matrix_out_path, "w", encoding="utf-8") as matrix_file_handler:
            logging.info("Writing testing matrix to '%s'", matrix_out_path)
            matrix_file_handler.write(self.matrix_str)
        for scenario, varfile in self.scenarios.items():
            varfile_output_path = output_path / VARS_DIR_PATH / (scenario + ".yml")
            with open(
                varfile_output_path, "w", encoding="utf-8"
            ) as varfile_file_handler:
                logging.info("Writing varfile '%s'", varfile_output_path)
                varfile_file_handler.write(varfile)


def get_argument_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "output_dir",
        help="""Directory to output results to. Will create the following files/dirs:
        - matrix.json: Testing matrix (if varfiles mode is used)
        - vars/: Ansible variable files passed to CircleCI workflow
        - pipeline.yml: CircleCI workflow (if pipeline mode is used)
        """,
    )
    parser.add_argument(
        "--just-varfiles",
        help="If given, only varfiles will be written",
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

    matrix_handler = MatrixHandler()
    matrix_handler.load()
    matrix_handler.write(output_dir)
    scenario_list = list(matrix_handler.scenarios.keys())

    if args.just_varfiles:
        return

    template_handler = TemplateHandler()
    template_handler.add_vars(
        build_image=args.build_image,
        scenarios=scenario_list,
        vars_dir=VARS_DIR_PATH,
    )
    template_handler.render_all()
    template_handler.write_all(output_dir)
    return


if __name__ == "__main__":
    main()
