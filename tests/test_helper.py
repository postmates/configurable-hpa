""" Helper functions to test CHPA """
import time
import json
import subprocess

def check_k8s_version():
    """ Check that kubectl can connect to the server and get version """
    output = subprocess.check_output(["kubectl", "version", "-o", "json"])
    server = json.loads(output)["serverVersion"]
    print("Kubernetes Server Version: {}.{}".format(server["major"], server["minor"]))

def run_manager_in_bg():
    """ Run CHPA controller (manager) in background """
    # manager should be built before running the tests
    # return subprocess.Popen(["../bin/manager"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    # For now skip running the manager, use the installed one
    return None

def stop_manager(_pipe):
    """ Stop CHPA controller (manager) that run in background """
    #pipe.kill()
    #return pipe.communicate()
    return (b'', b'')

def setup_cluster(name, label_key, label_value):
    """ Setup a cluster """
    cmd = ["kubectl", "run", name, "--image=k8s.gcr.io/hpa-example",
           "--requests=cpu=10m", "--expose", "--port=80",
           "--labels={}={},test={}".format(label_key, label_value, name)]
    check_output(cmd)

def teardown_cluster(name, _label_key, _label_value):
    """ Tear down the cluster """
    cmd = ["kubectl", "delete", "deploy,service,chpas.autoscalers.postmates.com", name]
    check_output(cmd)

def check_output(cmd):
    """ run command, print everything """
    print("Running: {}".format(" ".join(cmd)))
    try:
        output = subprocess.check_output(cmd, stderr=subprocess.STDOUT).decode("utf-8")
        print("Output: {}".format(output))
    except subprocess.CalledProcessError as err:
        print("Error code: {}".format(err.returncode))
        print("Error message:\n{}".format(err.output.decode("utf-8")))
        raise err

def get_deploy(name):
    """ get deployment """
    cmd = ["kubectl", "get", "deploy", name, "-o", "json"]
    output = subprocess.check_output(cmd)
    return json.loads(output)

def run_until(timeout, fun):
    """ run function until it returns True or timeout seconds passes """
    start = time.time()
    while True:
        if fun():
            return True
        if time.time() > start + timeout:
            break
        time.sleep(1)
    return False
