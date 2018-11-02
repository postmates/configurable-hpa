""" Helper module to deal with CHPA objects """

import copy
import os
import tempfile

class CHPA:
    """ Class to represent CHPA, store/load from/to file """
    allowed_options = ["refKind",
                       "downscaleForbiddenWindowSeconds", "upscaleForbiddenWindowSeconds",
                       "tolerance", "minReplicas", "targetCPUUtilizationPercentage"]
    default_options = {
        "labelKey": "app",
        "labelValue": "hpa-test",
        "apiVersion": "autoscalers.postmates.com/v1beta1",
        "kind": "CHPA",
        "refKind": "Deployment",
        "downscaleForbiddenWindowSeconds": 10,  # chpa-controller runs once per 15 sec
        "upscaleForbiddenWindowSeconds": 10,    # to fasten the test we should set timeout < 15sec
        "tolerance": 0.1,
        "minReplicas": 1,
        "targetCPUUtilizationPercentage": 80}

    format_str = """{{
    "apiVersion": "{apiVersion}",
    "kind": "{kind}",
    "metadata": {{
        "labels": {{"{labelKey}": "{labelValue}"}},
        "name": "{name}"
    }},
    "spec": {{
        "downscaleForbiddenWindowSeconds": {downscaleForbiddenWindowSeconds},
        "upscaleForbiddenWindowSeconds": {upscaleForbiddenWindowSeconds},
        "tolerance": {tolerance},
        "scaleTargetRef": {{
            "kind": "{refKind}",
            "name": "{refName}"
        }},
        "minReplicas": {minReplicas},
        "maxReplicas": {maxReplicas},
        "targetCPUUtilizationPercentage": {targetCPUUtilizationPercentage}
    }}
}}"""

    def __init__(self, name, maxReplicas, refName, options=None):
        """ Create a new CHPA instance"""
        if options is None:
            options = {}
        self.options = copy.copy(self.default_options)
        self.options.update({k: v for k, v in options.items() if k in self.allowed_options})
        self.options.update({"name": name,
                             "maxReplicas": maxReplicas,
                             "refName": refName})
        self.path = None

    def __repr__(self):
        return self.format_str.format(**self.options)

    def __del__(self):
        os.unlink(self.path)

    def save_to_tmp_file(self):
        """
        store the CHPA into a temporary file and return the path to the file
        you should remove the file after the test
        """
        if self.path is not None:
            # we already saved the file
            return self.path

        (tmphandle, tmppath) = tempfile.mkstemp(".json")
        with os.fdopen(tmphandle, "w") as tmpfile:
            tmpfile.write(repr(self))
        self.path = tmppath
        print("Created file {}:".format(self.path))
        with open(self.path, "r") as tmpfile:
            print(tmpfile.read())
        return self.path
