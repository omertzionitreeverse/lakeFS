# coding: utf-8

"""
    lakeFS API

    lakeFS HTTP API

    The version of the OpenAPI document: 0.1.0
    Contact: services@treeverse.io
    Generated by OpenAPI Generator (https://openapi-generator.tech)

    Do not edit the class manually.
"""  # noqa: E501


from __future__ import annotations
import pprint
import re  # noqa: F401
import json

from datetime import datetime
from typing import Optional
from pydantic import BaseModel, Field, StrictBool, StrictInt, StrictStr
from lakefs_sdk.models.commit import Commit
from lakefs_sdk.models.error import Error

class ImportStatus(BaseModel):
    """
    ImportStatus
    """
    completed: StrictBool = Field(...)
    update_time: datetime = Field(...)
    ingested_objects: Optional[StrictInt] = Field(None, description="Number of objects processed so far")
    metarange_id: Optional[StrictStr] = None
    commit: Optional[Commit] = None
    error: Optional[Error] = None
    __properties = ["completed", "update_time", "ingested_objects", "metarange_id", "commit", "error"]

    class Config:
        """Pydantic configuration"""
        allow_population_by_field_name = True
        validate_assignment = True

    def to_str(self) -> str:
        """Returns the string representation of the model using alias"""
        return pprint.pformat(self.dict(by_alias=True))

    def to_json(self) -> str:
        """Returns the JSON representation of the model using alias"""
        return json.dumps(self.to_dict())

    @classmethod
    def from_json(cls, json_str: str) -> ImportStatus:
        """Create an instance of ImportStatus from a JSON string"""
        return cls.from_dict(json.loads(json_str))

    def to_dict(self):
        """Returns the dictionary representation of the model using alias"""
        _dict = self.dict(by_alias=True,
                          exclude={
                          },
                          exclude_none=True)
        # override the default output from pydantic by calling `to_dict()` of commit
        if self.commit:
            _dict['commit'] = self.commit.to_dict()
        # override the default output from pydantic by calling `to_dict()` of error
        if self.error:
            _dict['error'] = self.error.to_dict()
        return _dict

    @classmethod
    def from_dict(cls, obj: dict) -> ImportStatus:
        """Create an instance of ImportStatus from a dict"""
        if obj is None:
            return None

        if not isinstance(obj, dict):
            return ImportStatus.parse_obj(obj)

        _obj = ImportStatus.parse_obj({
            "completed": obj.get("completed"),
            "update_time": obj.get("update_time"),
            "ingested_objects": obj.get("ingested_objects"),
            "metarange_id": obj.get("metarange_id"),
            "commit": Commit.from_dict(obj.get("commit")) if obj.get("commit") is not None else None,
            "error": Error.from_dict(obj.get("error")) if obj.get("error") is not None else None
        })
        return _obj

