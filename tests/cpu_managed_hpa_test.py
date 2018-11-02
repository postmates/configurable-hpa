""" Module to run all CPU-based autoscaling tests """

import sys
import unittest
import chpa
import test_helper

MANAGER_PIPE = None

def setUpModule(): # pylint: disable=invalid-name
    """ set up module-level stuff """
    global MANAGER_PIPE  # pylint: disable=global-statement
    test_helper.check_k8s_version()
    MANAGER_PIPE = test_helper.run_manager_in_bg() # pylint: disable=assignment-from-none

def tearDownModule(): # pylint: disable=invalid-name
    """ tear down module-level stuff """
    global MANAGER_PIPE  # pylint: disable=global-statement
    out, err = test_helper.stop_manager(MANAGER_PIPE)
    sys.stdout.write("stdout: " + out.decode("utf-8"))
    sys.stdout.write("stderr: " + err.decode("utf-8"))

class HPATestCase(unittest.TestCase):
    """ Class for all HPA autoscaling tests """
    DEPLOY_NAME_PREFIX = "chpa-test"
    DEPLOY_LABEL_KEY = "app"
    DEPLOY_LABEL_VALUE = "chpa-test"
    DEFAULT_TEST_TIMEOUT = 10 # seconds to wait for the test to pass

    @classmethod
    def setUpClass(cls):
        print("")
        name = "{}-{}".format(cls.DEPLOY_NAME_PREFIX, cls.__name__).lower()
        test_helper.setup_cluster(name, cls.DEPLOY_LABEL_KEY, cls.DEPLOY_LABEL_VALUE)

    @classmethod
    def tearDownClass(cls):
        print("")
        name = "{}-{}".format(cls.DEPLOY_NAME_PREFIX, cls.__name__).lower()
        test_helper.teardown_cluster(name, cls.DEPLOY_LABEL_KEY, cls.DEPLOY_LABEL_VALUE)

    def resource_name(self):
        """ any resource name to be used during this test """
        name = "{}-{}".format(self.DEPLOY_NAME_PREFIX, self.__class__.__name__).lower()
        return name

class TestMinReplicasAutoIncrease(HPATestCase):
    """ Class for all CPU-based autoscaling tests """

    def test_me(self):
        """ test something """
        name = self.resource_name()
        chpa_obj = chpa.CHPA(name, 3, name, {"minReplicas": 2})
        file_path = chpa_obj.save_to_tmp_file()
        test_helper.check_output(["kubectl", "apply", "-f", file_path])
        def check_replicas():
            deploy = test_helper.get_deploy(name)
            print("deploy status: {}".format(deploy["status"]))
            return deploy["status"]["replicas"] == 2
        res = test_helper.run_until(self.DEFAULT_TEST_TIMEOUT, check_replicas)
        self.assertTrue(res)

# use parallel approach
# https://stackoverflow.com/questions/4710142/can-pythons-unittest-test-in-parallel-like-nose-can

if __name__ == '__main__':
    unittest.main()
