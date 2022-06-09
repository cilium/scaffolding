#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Download artifacts of a CircleCI Job.

Provides interactive interface which allows a user to explore the
workflows within a project, the jobs within a workflow, and the artifacts
within a job.

This script can be used to download the 'context' needed to re-run a
playbook locally, or to just save and inspect artifacts as needed.

Uses the CircleCI API, which requires the following input:

* CircleCI API Key Token
* Project Name
"""
import base64
import concurrent.futures
import dataclasses
import http.client
import json
import queue
import sys
import threading
import typing as t
import urllib.error
import urllib.request
from pathlib import Path

import click
import questionary
import rich.progress
import rich.status
from typing_extensions import runtime_checkable

_JSON_TYPE = t.Union[t.List[t.Dict[str, t.Any]], t.Dict[str, t.Any]]


@runtime_checkable
class _DataclassProtocol(t.Protocol):
    """
    Define protocol for dataclass objects.

    Use as a type hint for dataclasses.
    """

    __dataclass_fields__: t.Dict
    __call__: t.Callable


@dataclasses.dataclass
class Me:
    """
    Response to the 'me' CircleCI API endpoint.
    """

    name: str
    login: str
    id: str


@dataclasses.dataclass
class Organization:
    """
    Response item to the 'collaborations' CircleCI API endpoint.
    """

    id: str
    vcs_type: str
    name: str
    avatar_url: str
    slug: str

    def __repr__(self) -> str:
        return self.slug


@dataclasses.dataclass
class ProjectVCSInfo:
    """
    Information regarding a VCS.

    Used in responses from other endpoints.
    """

    vcs_url: str
    provider: str
    default_branch: str


@dataclasses.dataclass
class Project:
    """
    Response to the 'project' CircleCI API endpoint.
    """

    slug: str
    name: str
    organization_name: str
    organization_slug: str
    organization_id: str
    id: t.Optional[str] = None
    vcs_info: t.Optional[ProjectVCSInfo] = None

    def __repr__(self) -> str:
        return self.name


@dataclasses.dataclass
class PipelineError:
    """
    Part of response to the 'pipeline' CircleCI API endpoint.

    See `Pipeline.errors`.
    """

    type: str
    message: str


@dataclasses.dataclass
class PipelineActor:
    """
    Part of `PipelineTrigger`.

    See `PipelineTrigger.actor`.
    """

    login: str
    avatar_url: str


@dataclasses.dataclass
class PipelineTrigger:
    """
    Part of response to the 'pipeline' CircleCI API endpoint.

    See `Pipeline.trigger`.
    """

    type: str
    received_at: str
    actor: PipelineActor


@dataclasses.dataclass
class PipelineCommit:
    """
    Part of `PipelineVCS`.

    See `PipelineVCS.commit`.
    """

    subject: str
    body: str


@dataclasses.dataclass
class PipelineVCS:
    """
    Part of response to the 'pipeline' CircleCI API endpoint.

    See `Pipeline.vcs`.
    """

    provider_name: str
    target_repository_url: str
    branch: str
    revision: str
    commit: PipelineCommit
    origin_repository_url: str
    review_id: t.Optional[str] = None
    review_url: t.Optional[str] = None
    tag: t.Optional[str] = None


@dataclasses.dataclass
class Pipeline:
    """
    Response item to the 'pipeline' CircleCI API endpoint.
    """

    id: str
    errors: t.List[PipelineError]
    project_slug: str
    updated_at: str
    number: int
    state: str
    created_at: str
    trigger: PipelineTrigger
    vcs: PipelineVCS
    trigger_parameters: t.Optional[t.Dict[str, str]] = None

    def __repr__(self) -> str:
        return f"{self.number} {self.created_at} {self.project_slug} ({self.id})"


@dataclasses.dataclass
class Workflow:
    """
    Response item to the 'workflow' CircleCI API endpoint.
    """

    pipeline_id: str
    id: str
    name: str
    project_slug: str
    status: str
    started_by: str
    pipeline_number: int
    created_at: str
    stopped_at: str
    canceled_by: t.Optional[str] = None
    errored_by: t.Optional[str] = None
    tag: t.Optional[str] = None

    def __repr__(self) -> str:
        return f"{self.created_at} {self.status.upper()} {self.name} ({self.id})"


@dataclasses.dataclass
class Job:
    """
    Response item to the `jobs` CircleCI API endpoint.
    """

    dependencies: t.List[str]
    id: str
    started_at: str
    name: str
    project_slug: str
    status: t.Optional[str]
    type: str
    job_number: t.Optional[int] = None
    stopped_at: t.Optional[str] = None
    approved_by: t.Optional[str] = None
    approval_request_id: t.Optional[str] = None
    canceled_by: t.Optional[str] = None

    def __repr__(self) -> str:
        return f"{self.name} ({self.id})"


@dataclasses.dataclass
class Artifact:
    """
    Response item to the `artifacts` CircleCI API endpoint.
    """

    path: str
    node_index: int
    url: str


@dataclasses.dataclass
class CircleApiContext:
    """
    Set CircleCI API parameters.

    Some API endpoints require that certain arguments are given, such
    as a project ID or organization slug. This dataclass represents
    said arguments and is used by the `CircleApiHandler` to make
    requests.
    """

    api_key: t.Optional[str] = None
    org: t.Optional[Organization] = None
    project: t.Optional[Project] = None
    pipeline: t.Optional[Pipeline] = None
    workflow: t.Optional[Workflow] = None
    job: t.Optional[Job] = None

    def has(self, *args: str) -> t.List[str]:
        """
        Check if the given attributes are not None.

        Parameters
        ----------
        *args : str
            List of attribute names as strs to check presence of.

        Returns
        -------
        List of missing attributes
        """

        missing = []
        dne = "does_not_exist"
        for attr in args:
            val = getattr(self, attr, dne)
            if val == dne:
                raise ValueError(f"Unknown attribute of CircleApiContext: {attr}")
            if val is None:
                missing.append(attr)
        return missing


class CircleApiHandler:
    """
    Make requests to the CircleCI API and decode responses.

    Any method which starts with `get` or `set` involves
    setting the context for the handler, which determines what
    information can be used in future API calls.

    Responses to the API are cached. Note that if a variable in
    the context is changed the cache will not be invalidated, as
    a builder-like pattern is assumed here.

    To ignore the cache and force an update, pass `no_cache`
    to an API method.

    Parameters
    ----------
    context : CircleApiContext
        CircleApiContext to use for setting arguments on API
        calls.
    """

    API_URL_PREFIX = "/api/v2/"

    def __init__(self, context: CircleApiContext):
        """Create the CircleApiHandler"""

        self._cache: t.Dict[str, t.Any] = {}
        self.context = context

    def _add_to_cache(self, name: str, value: t.Any):
        """
        Add value into the cache under given name.

        Parameters
        ----------
        name : str
            Key in the cache
        value : Any
            Value in the cache
        """

        self._cache[name] = value

    def _from_cache(self, name: str) -> t.Any:
        """
        Get value from the cache under given name.

        Returns None if the name is not set in the cache.

        Parameters
        ----------
        name : str
            Key in the cache

        Returns
        -------
        Value in the cache, None
        """

        return self._cache.get(name, None)

    @staticmethod
    def _parse_response(res: http.client.HTTPResponse) -> t.Dict[str, t.Any]:
        """
        Parse the response given in the HTTPResponse object.

        Attempts to decode the response as JSON, falling back to the
        following format in case of a failure:

            {"response": content-body}

        Parameters
        ----------
        res : http.client.HTTPResponse
            HTTPResponse object to parse content from

        Returns
        -------
        dict
        """

        content_bytes = res.read()
        try:
            content = content_bytes.decode("utf-8")
        except UnicodeDecodeError:
            return {"response": content_bytes}

        try:
            return json.loads(content)
        except json.decoder.JSONDecodeError:
            return {"response": content}

    def _do_request(self, verb: str, endpoint: str, api_key: str) -> _JSON_TYPE:
        """
        Perform a request to the circleci API server.

        Parameters
        ----------
        verb : str
            HTTP verb to use.
        endpoint : str
            HTTP endpoint to hit.
        api_key : str
            Value set under the 'authorization' header in the request.

        Return
        ------
        JSON decoded response.
        """
        auth = base64.b64encode(api_key.encode("utf-8")).decode("utf-8")
        headers = {"Authorization": f"Basic {auth}", "Content-Type": "application/json"}
        conn = http.client.HTTPSConnection("circleci.com")
        conn.request(verb, endpoint, headers=headers)

        res = conn.getresponse()
        return self._parse_response(res)

    @staticmethod
    def _get_field_names_from_dc(
        dataclass: _DataclassProtocol, exclude_optional: bool = True
    ) -> t.Tuple[str]:
        """
        Return the names of the fields in the given dataclass.

        Parameters
        ----------
        dataclass : dataclass
        exclude_optional : bool
            If True, will exclude Fields marked as optional within
            the returned Tuple

        Returns
        -------
        tuple of str
        """

        return tuple(
            f.name
            for f in dataclasses.fields(dataclass)
            if not (getattr(f.type, "_name", None) == "Optional" and exclude_optional)
        )

    @staticmethod
    def _validate_resp_keys(resp: t.Dict[str, t.Any], keys: t.Iterable[str]):
        """
        Validate that the resp dict has the given keys.

        Used to validate the format of responses from the API
        server.

        Raises
        ------
        ValueError
            If the validation was not successful.
        """

        received_keys = list(resp.keys())
        if sorted(received_keys) != sorted(keys):
            extra = [key for key in received_keys if key not in keys]
            missing = [key for key in keys if key not in received_keys]

            if len(missing) == 0 and len(extra) > 0:
                return  # probably just given an optional field

            missing_str = ", ".join([f'"{key}"' for key in missing])

            raise ValueError(
                f"Response does not match expected object, missing: {missing_str}, response: {resp}"
            )

    def _construct_resp(self, resp: _JSON_TYPE, dataclass: _DataclassProtocol) -> t.Any:
        """
        Construct given response into dataclasses recursively.

        Validates the response along the way.

        Parameters
        ----------
        resp : dict
            Response to validate and parse
        dataclass : dataclass
            Top-level dataclass the response is fitted into.
        """

        if isinstance(resp, list):
            return [self._construct_resp(item, dataclass) for item in resp]
        
        # Handle paginated responses
        # Currently ignore need for making another request using the 
        # next page token, as for our use cases we should be able
        # to fit all our results for each endpoint in one page
        if "items" in resp.keys() and "next_page_token" in resp.keys():
            return self._construct_resp(resp["items"], dataclass)

        self._validate_resp_keys(resp, self._get_field_names_from_dc(dataclass))

        dc_fields = dataclasses.fields(dataclass)
        args = resp.copy()
        for field in dc_fields:
            if isinstance(field.type, _DataclassProtocol):
                args[field.name] = self._construct_resp(resp[field.name], field.type)
            # If a field has a type of t.List[_DataclassProtocol], then it's
            # __origin__ attribute will be set to 'list' and it's __args__
            # attribute will contain the _DataclassProtocol as the first item.
            elif getattr(field.type, "__origin__", None) is list and isinstance(
                field.type.__args__[0], _DataclassProtocol
            ):
                args[field.name] = self._construct_resp(
                    resp[field.name], field.type.__args__[0]
                )
        return dataclass(**args)

    @staticmethod
    def _cached(func):
        """
        Decorator to handle caching of a method's result.

        Uses the method's string name as the key.
        """

        def inner(self, *args, no_cache: bool = False, **kwargs):
            if no_cache:
                return func(self, *args, **kwargs)

            name = func.__name__
            cached = self._from_cache(name)
            if cached is not None:
                return cached

            result = func(self, *args, **kwargs)
            self._add_to_cache(name, result)
            return result

        inner.__name__ = func.__name__
        return inner

    @staticmethod
    def _requires(*reqs: str):
        """uee
        Decorator ensuring given attrs are set in the context.

        If you see a bunch of `# type: ignore` in a method
        wrapper with this decorator, it is because some type
        checkers just aren't smart enough to know that this
        function ensures context attributes are set.

        Raises
        ------
        ValueError
            If a given attribute is not set in the context.
        """

        def wrap(func):
            def inner(self, *args, **kwargs):
                name = func.__name__
                missing = self.context.has(*reqs)
                if len(missing) > 0:
                    raise ValueError(
                        f"Expected {', '.join(missing)} to be set before calling {name}"
                    )
                return func(self, *args, **kwargs)

            return inner

        return wrap

    @_requires("api_key")
    @_cached
    def me(self) -> Me:
        """
        Provides information about the user currently signed in.

        https://circleci.com/docs/api/v2/#operation/getCurrentUser

        Raises
        ------
        ValueError
            If the given response does not have the same fields
            as the `Me` dataclass.

        Returns
        -------
        Me
        """

        result = self._do_request(
            "GET",
            self.API_URL_PREFIX + "me",
            self.context.api_key,  # type: ignore
        )
        return self._construct_resp(result, Me)

    @_requires("api_key")
    @_cached
    def orgs(self) -> t.List[Organization]:
        """
        Provides information about orgs current user is a part of.

        https://circleci.com/docs/api/v2/#operation/getCollaborations

        Raises
        ------
        ValueError
            If the given response cannot be turned into a list of
            `Organization` dataclasses.
        
        Returns
        -------
        List of Organizations
        """

        result = self._do_request(
            "GET",
            self.API_URL_PREFIX + "me/collaborations",
            self.context.api_key,  # type: ignore
        )

        return self._construct_resp(result, Organization)

    @_requires("api_key")
    @_cached
    def org(self, slug: str) -> Organization:
        """
        Provides information about given organization.

        Parameters
        ----------
        slug : str
            Return organization with given slug.

        Raises
        ------
        ValueError
            If org with given slug cannot be found.
        """

        orgs = self.orgs()
        for org in orgs:
            if org.slug == slug:
                return org

        raise ValueError(f"Could not find org with given slug {slug}")

    @_requires("api_key", "org")
    @_cached
    def project(self, name: str) -> Project:
        """
        Provides information about given project.

        https://circleci.com/docs/api/v2/#operation/getProjectBySlug

        Parameters
        ----------
        name : str
            Return project with given name,
            using organization from current context.

        Raises
        ------
        ValueError
            If the given response cannot be turned into a
            `Project` dataclass.

        Returns
        -------
        Project
        """

        result = self._do_request(
            "GET",
            self.API_URL_PREFIX + f"project/{self.context.org.slug}/{name}",  # type: ignore
            self.context.api_key,  # type: ignore
        )

        return self._construct_resp(result, Project)

    @_requires("api_key", "org", "project")
    @_cached
    def pipelines(self) -> t.List[Pipeline]:
        """
        Provide list of pipelines within the current Project.

        https://circleci.com/docs/api/v2/#operation/listPipelinesForProject

        Raises
        ------
        ValueError
            If the given response cannot be turned into a
            list of `Pipeline` dataclasses.

        Returns
        -------
        list of Pipeline
        """

        result = self._do_request(
            "GET",
            self.API_URL_PREFIX + f"project/{self.context.project.slug}/pipeline",  # type: ignore
            self.context.api_key,  # type: ignore
        )

        return self._construct_resp(result, Pipeline)

    @_requires("api_key", "org", "project")
    @_cached
    def pipeline(self, id: str) -> Pipeline:
        """
        Provides information about given pipeline.

        Parameters
        ----------
        id : str
            Return pipeline with given id.

        Raises
        ------
        ValueError
            If pipeline with given id cannot be found.
        """

        pipelines = self.pipelines()
        for pipeline in pipelines:
            if pipeline.id == id:
                return pipeline
        raise ValueError(f"Cannot find pipeline with given id {id}")

    @_requires("api_key", "pipeline")
    @_cached
    def workflows(self) -> t.List[Workflow]:
        """
        Provide list of Workflows within current Pipeline.

        https://circleci.com/docs/api/v2/#operation/listWorkflowsByPipelineId

        Raises
        ------
        ValueError
            If the given response cannot be turned into a list of
            `Workflow` dataclasses.

        Returns
        -------
        List of Workflow
        """

        result = self._do_request(
            "GET",
            self.API_URL_PREFIX + f"pipeline/{self.context.pipeline.id}/workflow",  # type: ignore
            self.context.api_key,  # type: ignore
        )

        return self._construct_resp(result, Workflow)

    @_requires("api_key", "pipeline")
    @_cached
    def workflow(self, id: str) -> Workflow:
        """
        Provides information about given workflow.

        Parameters
        ----------
        id : str
            Return workflow with given id.

        Raises
        ------
        ValueError
            If workflow with given id cannot be found.
        """

        workflows = self.workflows()
        for workflow in workflows:
            if workflow.id == id:
                return workflow
        raise ValueError(f"Cannot find workflow with given id {id}")

    @_requires("api_key", "workflow")
    @_cached
    def jobs(self) -> t.List[Job]:
        """
        Provide list of Jobs within current Workflow.

        https://circleci.com/docs/api/v2/#operation/listWorkflowJobs

        Raises
        ------
        ValueError
            If the given response cannot be turned into a list of
            `Jobs` dataclasses

        Returns
        -------
        List of Jobs
        """

        result = self._do_request(
            "GET",
            self.API_URL_PREFIX + f"workflow/{self.context.workflow.id}/job",  # type: ignore
            self.context.api_key,  # type: ignore
        )

        return self._construct_resp(result, Job)

    @_requires("api_key", "workflow")
    @_cached
    def job(self, id: str) -> Job:
        """
        Provides information about given job.

        Parameters
        ----------
        id : str
            Return job with given id.

        Raises
        ------
        ValueError
            If job with given id cannot be found.
        """

        jobs = self.jobs()
        for job in jobs:
            if job.id == id:
                return job
        raise ValueError(f"Cannot find job with given id {id}")

    @_requires("api_key", "project", "job")
    @_cached
    def artifacts(self) -> t.List[Artifact]:
        """
        Provide list of Artifacts within the current Job.

        https://circleci.com/docs/api/v2/#operation/getJobArtifacts

        Raises
        ------
        ValueError
            If the given response cannot be turned into a
            list of `Artifact` dataclasses

        Returns
        -------
        List of Artifacts
        """

        result = self._do_request(
            "GET",
            self.API_URL_PREFIX
            + f"project/{self.context.project.slug}/{self.context.job.job_number}/artifacts",  # type: ignore
            self.context.api_key,  # type: ignore
        )

        return self._construct_resp(result, Artifact)


class Prompt:
    """
    Prompt manager used for interacting with users.

    Allows users to work through setting context hierarchy to eventually
    get to the point of downloading artifacts. Walks the user through
    selecting the following, in the following order:

    1. Organization
    2. Project
    3. Pipeline
    4. Workflow
    5. Job

    These items can be provided ahead of time rather than having
    to be provided by users every time.
    """

    PROMPT_DEPENDENCY_MAP = {
        None: ["job"],
        "job": ["workflow"],
        "workflow": ["pipeline"],
        "pipeline": ["project"],
        "project": ["org"],
        "org": ["say_hi"],
    }
    ITEM_SELECT_INFO = {
        "job": {
            "identifier": "id",
            "list_method": "jobs",
            "prompt": "🚂 Which job would you like?",
        },
        "workflow": {
            "identifier": "id",
            "list_method": "workflows",
            "prompt": "🔄 Which workflow?",
        },
        "pipeline": {
            "identifier": "id",
            "list_method": "pipelines",
            "prompt": "🧪 Which pipeline are we using?",
        },
        "project": {
            "identifier": "name",
            "list_method": "",
            "prompt": "📝 What's the project?",
        },
        "org": {
            "identifier": "slug",
            "list_method": "orgs",
            "prompt": "🏢 Which org do you want to use?",
        },
    }

    def __init__(
        self,
        api_key: str,
        org_slug: t.Optional[str] = None,
        project_name: t.Optional[str] = None,
        pipeline_id: t.Optional[str] = None,
        workflow_id: t.Optional[str] = None,
        job_id: t.Optional[str] = None,
    ):

        self.givens = {
            "org": org_slug,
            "project": project_name,
            "pipeline": pipeline_id,
            "workflow": workflow_id,
            "job": job_id,
        }

        self.api = CircleApiHandler(CircleApiContext(api_key=api_key))

        self.progress = rich.progress.Progress(
            rich.progress.TextColumn("{task.description}"),
            rich.progress.TextColumn(
                "[bold blue]{task.fields[filename]}", justify="left"
            ),
            rich.progress.BarColumn(bar_width=None),
            "[progress.percentage]{task.percentage:>3.1f}%",
            "*",
            rich.progress.DownloadColumn(),
            "*",
            rich.progress.TransferSpeedColumn(),
            "*",
            rich.progress.TimeRemainingColumn(),
            transient=True,
        )

    @staticmethod
    def print_error_and_exit(header: str, msg: t.Optional[str] = None):
        """
        Print the given error header and message and then exit with rc 1.
        """

        questionary.print(
            "🚨 " + header + (":" if msg is not None else ""), style="bold"
        )
        if msg:
            questionary.print(msg)
        sys.exit(1)

    def error_invalid_choice(self, name: str, chosen: str, options: t.List[str]):
        """
        Provide error to the user that their given choice was invalid.

        ----------
        name : str
            Name of the 'thing' that wasn't chosen correctly.
        chosen : str
            Option chosen by the user.
        options : list of str
            Available options the user can choose.
        """

        self.print_error_and_exit(
            f'Could not find {name} "{chosen}"',
            f"Valid choices are: {', '.join(options)}",
        )

    def get_options(
        self, name: str, api_method: t.Callable[[], t.Any]
    ) -> t.List[t.Any]:
        """
        Get a list of option choices by making an API call.

        If the API call fails, will exit. Essentially just a wrapper
        around the method that handles a `ValueError` as needed.

        Parameters
        ----------
        name : str
            Name of the 'thing' being pulled from the API.
        api_method : callable
            API method to use to retrieve the choices.

        Returns
        -------
        List of items retrieved from the API.
        """

        try:
            result = api_method()
        except ValueError as err:
            self.print_error_and_exit(
                f"Unable to grab list of available {name}s", str(err)
            )

        items = getattr(result, "items", None)
        if items is not None:
            return items
        return result

    def resolve(self, name: str, given_value: str) -> t.Any:
        """
        Resolves a context item using the given value.

        Uses the CircleApiHandler to run a 'get' call against
        the given item type, passing the `given_value` as an
        identifier. This allows for an ID, slug, name, etc. to
        be resolved into the corresponding context item object.
        For instance, to resolve an organization by its slug:

        `self.resolve("org", "slug")`

        This will return the value of:

        `CircleApiHandler.org("slug")`

        If the resolution fails, then an error will be displayed
        to the user and the program will quit.

        Parameters
        ----------
        name : str
            Name of the context item to resolve.
        given_value : str
            Identifying value of the context item.
        """

        questionary.print(f"Checking {name} ({given_value})...")

        try:
            return getattr(self.api, name)(given_value)
        except ValueError:
            options_method = getattr(
                self.api, self.ITEM_SELECT_INFO[name]["list_method"], None
            )
            if options_method is not None:
                self.error_invalid_choice(
                    name, given_value, self.get_options(name, options_method)
                )
            else:
                self.print_error_and_exit(f"Unable to find {name} '{given_value}'")

    def say_hi(self):
        """
        Print name of user from api token.

        Asserts that the given api token can be used successfully.
        """

        try:
            name = self.api.me().name
        except ValueError as err:
            self.print_error_and_exit(
                "Unable to authenticate using provided token", str(err)
            )

        questionary.print(f"👋 Hey {name}!", style="bold")

    def get_item(
        self, prompt: str, name: str, items: t.Optional[t.List[t.Any]] = None
    ) -> t.Any:
        """
        Ask the user to select one of the given items.

        If items are not given, then asks the user to input text instead
        Input text is then passed through `resolve` in order validate
        the input is good.

        Parameters
        ----------
        prompt : str
            Prompt to display to the user
        name : str
            Name of the 'thing' being chosen by the user.
        items : list of objects, optional
            The object's `__repr__` method will be used
            to format each object into a string for the user to see.

        Returns
        -------
        Selected item
        """

        if items is None:
            return self.resolve(name, questionary.text(prompt).ask())

        option_map = {str(item): item for item in items}
        selected = questionary.select(prompt, choices=list(option_map.keys())).ask()
        if selected is None:  # got a keyboard interrupt
            raise KeyboardInterrupt()
        return option_map[selected]

    def fill_context(self, name: t.Optional[str] = None):
        """
        Fill in and validate items in the CircleApiContext.

        Assume that these items were set from user input on the
        command line and work needs to be done to: (1) ensure that
        all required items are given and (2) all items that were
        given actually exist.

        If an item is not given, then an attempt will be made
        to pull one from the user.

        Works recursively, starting with Artifacts and moving
        backwards.

        Parameters
        ----------
        name : str
            Item name to validate. Can be one of: 'artifacts',
            'job_id', 'workflow_id', 'pipeline_id', 'project',
            'organization'.
        """

        if name is None:  # start with jobs
            self.fill_context("job")
            return
        if name == "say_hi":  # test api key
            self.say_hi()
            return

        for dep in self.PROMPT_DEPENDENCY_MAP[name]:
            self.fill_context(dep)

        item_info = self.ITEM_SELECT_INFO[name]
        api_method = getattr(self.api, item_info["list_method"], None)
        if api_method is not None:
            options = self.get_options(name, api_method)
        else:
            options = None

        currently_set = getattr(self.api.context, name, None)
        if currently_set is None:
            given_value = self.givens[name]
            if given_value is not None:
                setattr(
                    self.api.context,
                    name,
                    self.resolve(
                        name,
                        given_value,
                    ),
                )
                return

            choice = self.get_item(item_info["prompt"], name, options)
            setattr(self.api.context, name, choice)

    def save_artifact_urls(self, url_file: Path) -> None:
        """
        Write newline-separated artifact urls into given file.

        Parameter
        ---------
        url_file : Path
            Location where artifact urls will be written to.
        """
        artifacts = self.get_options("artifact", self.api.artifacts)
        url_path = Path(url_file).resolve()
        questionary.print(
            f"Writing {len(artifacts)} urls to {str(url_path)}", style="bold"
        )

        with open(url_file, "w", encoding="utf-8") as url_file_handler:
            for artifact in artifacts:
                url_file_handler.write(f"{artifact.url}\n")

    def _download_artifact(
        self,
        task_id: rich.progress.TaskID,
        artifact: Artifact,
        dest_dir: Path,
        artifact_semaphore: threading.Semaphore,
        run_event: threading.Event,
    ) -> None:
        """
        Download url to the given path, showing progress along the way.

        Attempt to handle errors as they can occur, including interrupts.
        """

        def handle_error(err):
            self.progress.update(
                task_id=task_id, description="[bold red] FAIL", refresh=True
            )
            self.progress.console.print(
                f"🚨 Unable to download {artifact.url} to {str(dest_dir)}:[bold red] {str(err)}"
            )
            self.progress.stop_task(task_id)
            artifact_semaphore.release()

        def check_run_event():
            if not run_event.is_set():
                raise InterruptedError

        check_run_event()

        try:
            path = (dest_dir / artifact.path).resolve()
            path.parent.mkdir(parents=True, exist_ok=True)
        except OSError as err:
            handle_error(err)
            raise

        try:
            response = urllib.request.urlopen(artifact.url)
        except urllib.error.URLError as err:
            handle_error(err)
            raise

        content_length = response.info().get("Content-length", None)
        if content_length is None:
            err = ValueError("Could not get content-length")
            handle_error(err)
            raise err
        self.progress.update(task_id, total=int(content_length))

        try:
            with open(path, "wb") as dest_file:
                self.progress.start_task(task_id)
                # Choosing 1000 bytes allow for progress bar
                # to update frequently
                data = response.read(1000)
                while len(data) > 0:
                    check_run_event()
                    dest_file.write(data)
                    self.progress.update(task_id, advance=len(data))
                    data = response.read(1000)
        except InterruptedError:
            raise
        except Exception as err:
            handle_error(err)
            raise
        finally:
            response.close()

        self.progress.print(f"[bold green] {artifact.path}")
        self.progress.stop_task(task_id)
        self.progress.remove_task(task_id)
        artifact_semaphore.release()

    def download_artifacts(self, dest_dir: Path, num_workers: int = 4) -> None:
        """
        Download all artifacts into the destination directory.

        Assumes context is filled.

        Parameters
        ----------
        dest_dir : str
            Directory to download artifacts into.
        num_workers : int
            Number of 'downloader' worker threads to spawn.
        """

        # Artifact semaphore controls how many jobs are placed
        # into the thread pool at once
        artifact_semaphore = threading.Semaphore(value=num_workers * 2)

        # run_event controls if the thread should continue executing
        run_event = threading.Event()
        run_event.set()

        artifacts = self.get_options("artifact", self.api.artifacts)
        questionary.print(
            f"💾 Downloading {len(artifacts)} artifacts...", style="italic"
        )
        dest_dir = dest_dir.resolve()

        with self.progress:
            with concurrent.futures.ThreadPoolExecutor(max_workers=num_workers) as pool:
                try:
                    for artifact in artifacts:
                        artifact_semaphore.acquire()
                        task_id = self.progress.add_task(
                            "[bold yellow] ...",
                            filename=artifact.path,
                            start=False,
                        )
                        pool.submit(
                            self._download_artifact,
                            task_id,
                            artifact,
                            dest_dir,
                            artifact_semaphore,
                            run_event,
                        )
                except KeyboardInterrupt:
                    run_event.clear()


@click.command()
@click.argument("token")
@click.option(
    "--token-file",
    is_flag=True,
    default=False,
    help="Treat given token as a file containing the token, rather than a token in its own right.",
)
@click.option("--org", default=None, help="Provide org to browse under.")
@click.option("--project", default=None, help="Provide project to browse under.")
@click.option(
    "--pipeline-id", default=None, help="Provide pipeline ID to browse under."
)
@click.option(
    "--workflow-id", default=None, help="Provide workflow ID to browse under."
)
@click.option("--job-id", default=None, help="Provide job ID to browse under.")
@click.option(
    "--output-dir",
    default=None,
    help="Destination directory for storing downloaded artifacts. Defaults to current directory.",
)
@click.option(
    "--url-file", default=None, help="Place URLs of all artifacts into given file."
)
@click.option(
    "--download/--no-download",
    default=True,
    help="Download artifacts from the selected job.",
)
@click.option(
    "--print-cli-options/--no-print-cli-options",
    default=False,
    help="Print out CLI options which would make the same selections as interactively.",
)
@click.option(
    "--absolute-cli-paths/--no-absolute-cli-paths",
    default=True,
    help="Resolve and print absolute paths in output of '--print-cli-options'",
)
def main(
    token,
    token_file,
    org,
    project,
    pipeline_id,
    workflow_id,
    job_id,
    output_dir,
    url_file,
    download,
    print_cli_options,
    absolute_cli_paths,
):
    given_token = token
    if token_file:
        token = Path(token).read_text().strip()
    if output_dir is None:
        output_dir = Path(".")

    if url_file is not None:
        url_file = Path(url_file)

    prompt = Prompt(
        api_key=token,
        org_slug=org,
        project_name=project,
        pipeline_id=pipeline_id,
        workflow_id=workflow_id,
        job_id=job_id,
    )
    prompt.fill_context()

    if print_cli_options:
        cli_args = []

        if token_file:
            token_path = Path(given_token)
            cli_args.append(
                f"{token_path.absolute() if absolute_cli_paths else token_path}"
            )
            cli_args.append("--token-file")
        else:
            cli_args.append(given_token)
            cli_args.append("--no-token-file")

        if url_file is not None:
            cli_args.append(
                f"--url-file={url_file.absolute() if absolute_cli_paths else url_file}"
            )

        cli_args.append(
            f"--output-dir={output_dir.absolute() if absolute_cli_paths else output_dir}"
        )

        if download:
            cli_args.append("--download")
        else:
            cli_args.append("--no-download")

        if absolute_cli_paths:
            cli_args.append("--absolute-cli-paths")
        else:
            cli_args.append("--no-absolute-cli-paths")

        cli_args.append("--print-cli-options")

        context_to_cli = {
            "org": "--org",
            "project": "--project",
            "pipeline": "--pipeline-id",
            "workflow": "--workflow-id",
            "job": "--job-id",
        }

        for context_name, cli_arg in context_to_cli.items():
            context_obj = getattr(prompt.api.context, context_name)
            cli_args.append(
                cli_arg
                + "="
                + getattr(
                    context_obj,
                    prompt.ITEM_SELECT_INFO[context_name]["identifier"],
                )
            )
        questionary.print("📌 Use these options for next time: ", style="bold")
        questionary.print(" \\\n".join(cli_args))

    if url_file is not None:
        prompt.save_artifact_urls(url_file.absolute())
    if download:
        prompt.download_artifacts(output_dir.absolute())


if __name__ == "__main__":
    main()
