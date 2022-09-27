@Library('jenkins-library@main') _
dockerbuildtestPipeline(
  script: this,
  service: 'worker-operator',
  buildArguments: [PLATFORM:"amd64"],
  run_unit_tests: 'false'
)
