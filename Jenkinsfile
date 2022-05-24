@Library('jenkins-library@unit-opensource') _
dockerbuildtestPipeline(
  script: this,
  service: 'worker-operator',
  dockerfile: 'test.Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"]
)
