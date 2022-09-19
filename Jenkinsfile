@Library('jenkins-library@opensource-helm-pipeline-release') _
dockerbuildtestPipeline(
  script: this,
  service: 'worker-operator',
  buildArguments: [PLATFORM:"amd64"],
  run_unit_tests: 'true'
)
