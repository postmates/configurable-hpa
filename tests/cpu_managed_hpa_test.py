""" Module to run all CPU-based autoscaling tests """

import datetime
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
    cmd = ["kubectl", "delete",
           "service,deploy,chpas.autoscalers.postmates.com",
           "-l", "app=chpa-test"]
    test_helper.check_output(cmd)

class HPATestCase(unittest.TestCase):
    """ Class for all HPA autoscaling tests """
    DEPLOY_NAME_PREFIX = "chpa-test"
    DEPLOY_LABEL_KEY = "app"
    DEPLOY_LABEL_VALUE = "chpa-test"
    DEFAULT_TEST_TIMEOUT = 10 # seconds to run usual tests
    LONG_TEST_TIMEOUT = 300 # seconds to run long tests

    @classmethod
    def setUpClass(cls):
        print("")
        print("Run Test " + cls.__name__)
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

    def add_cpu_load(self, sleep):
        """ Add load to our chpa-managed deployment"""
        service_name = "{}-{}".format(self.DEPLOY_NAME_PREFIX, self.__class__.__name__).lower()
        name = "{}-load".format(service_name)
        command_str = "while true; do echo 'next'; wget -q -O- {}; sleep {}; done;"
        command = command_str.format(service_name, sleep)
        labels = "--labels={}={}".format(self.DEPLOY_LABEL_KEY, self.DEPLOY_LABEL_VALUE)
        cmd = ["kubectl", "run", name, "--image=busybox", labels, "--", "/bin/sh", "-c", command]
        test_helper.check_output(cmd)

    def remove_cpu_load(self):
        """ Remove deployment that adds load to our chpa-managed deployment"""
        name = "{}-{}-load".format(self.DEPLOY_NAME_PREFIX, self.__class__.__name__).lower()
        cmd = ["kubectl", "delete", "deploy", name]
        test_helper.check_output(cmd)

def check_replicas(name, num):
    """ return function that compares number of replicas to some number"""
    def fun():
        deploy = test_helper.get_deploy(name)
        print("deploy replicas: {}  (waiting {})".format(deploy["status"]["replicas"], num))
        return deploy["status"]["replicas"] == num
    return fun


class TestMinReplicasAutoIncrease(HPATestCase):
    """ Class for all CPU-based autoscaling tests """

    def test_me(self):
        """ test something """
        name = self.resource_name()
        chpa_obj = chpa.CHPA(name, 3, name, {"minReplicas": 2},)
        test_helper.check_output(["kubectl", "apply", "-f", chpa_obj.save_to_tmp_file()])

        res = test_helper.run_until(self.DEFAULT_TEST_TIMEOUT, check_replicas(name, 2))
        self.assertTrue(res)

class TestRaiseToMax(HPATestCase):
    """ Class for all CPU-based autoscaling tests """

    def test_me(self):
        """ test something """
        name = self.resource_name()
        chpa_obj = chpa.CHPA(name, 8, name, {"targetCPUUtilizationPercentage": 10})
        test_helper.check_output(["kubectl", "apply", "-f", chpa_obj.save_to_tmp_file()])

        self.add_cpu_load(0.5)
        # first scale up will be 1 -> 4
        res = test_helper.run_until(self.LONG_TEST_TIMEOUT, check_replicas(name, 4))
        self.assertTrue(res)
        # next will be 4 -> 8
        res = test_helper.run_until(self.LONG_TEST_TIMEOUT, check_replicas(name, 8))
        self.assertTrue(res)

        self.remove_cpu_load()
        # then it will go down instantly 8 -> 1
        res = test_helper.run_until(self.LONG_TEST_TIMEOUT, check_replicas(name, 1))
        self.assertTrue(res)

class TestRaiseToMaxFast(HPATestCase):
    """ Class for all CPU-based autoscaling tests """

    def test_me(self):
        """ test something """
        name = self.resource_name()
        chpa_obj = chpa.CHPA(name, 8, name, {"scaleUpLimitFactor": 10.0,
                                             "targetCPUUtilizationPercentage": 10})
        test_helper.check_output(["kubectl", "apply", "-f", chpa_obj.save_to_tmp_file()])

        self.add_cpu_load(0.5)
        # scaling will be very fast: 1 -> 8
        self.assertTrue(test_helper.run_until(self.LONG_TEST_TIMEOUT, check_replicas(name, 8)))

        self.remove_cpu_load()
        # then it will go down instantly 8 -> 1
        res = test_helper.run_until(self.LONG_TEST_TIMEOUT, check_replicas(name, 1))
        self.assertTrue(res)

class TestIncorrectCHPAs(HPATestCase):
    """ Class for all CPU-based autoscaling tests """

    def test_me(self):
        """ test something """
        name = self.resource_name()
        chpa_obj = chpa.CHPA(name, 3, name, {"minReplicas": 2})
        test_helper.check_output(["kubectl", "apply", "-f", chpa_obj.save_to_tmp_file()])

        res = test_helper.run_until(self.DEFAULT_TEST_TIMEOUT, check_replicas(name, 2))
        self.assertTrue(res)

# use parallel approach
# https://stackoverflow.com/questions/4710142/can-pythons-unittest-test-in-parallel-like-nose-can

if __name__ == '__main__':
    unittest.main()
