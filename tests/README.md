# E2E test for Configurable HPA

Tests are run for the `admin.us-east-2.aws.k8s` kubectl context.

*NB:* All the tests are sensitive to the path they are run from. Here's a correct way to run tests:

    cd tests
    python -m unittest *.py

The tests does the following:

- Create a CRD (`kubectl apply -f config/crds`)
- Run a CHPA controller in a separate thread (`make run &`)
- Run a list of tests. For each test:
  - Create a deployment + CHPA for the deployment
  - Increase a load for the deployment
  - Check that number of replicas has increased
  - Decrease a load for the deployment
  - Check that number of replicas has decreased
  - Delete a deployment + CHPA for the deployment
- Stop the CHPA controller
