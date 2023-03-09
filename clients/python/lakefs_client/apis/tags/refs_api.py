# coding: utf-8

"""
    lakeFS API

    lakeFS HTTP API  # noqa: E501

    The version of the OpenAPI document: 0.1.0
    Contact: services@treeverse.io
    Generated by: https://openapi-generator.tech
"""

from lakefs_client.paths.repositories_repository_refs_left_ref_diff_right_ref.get import DiffRefs
from lakefs_client.paths.repositories_repository_refs_dump.put import DumpRefs
from lakefs_client.paths.repositories_repository_refs_source_ref_merge_destination_branch.get import FindMergeBase
from lakefs_client.paths.repositories_repository_refs_ref_commits.get import LogCommits
from lakefs_client.paths.repositories_repository_refs_source_ref_merge_destination_branch.post import MergeIntoBranch
from lakefs_client.paths.repositories_repository_refs_restore.put import RestoreRefs


class RefsApi(
    DiffRefs,
    DumpRefs,
    FindMergeBase,
    LogCommits,
    MergeIntoBranch,
    RestoreRefs,
):
    """NOTE: This class is auto generated by OpenAPI Generator
    Ref: https://openapi-generator.tech

    Do not edit the class manually.
    """
    pass