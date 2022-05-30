@Library('jenkins-library@feature-opensource') _
dockerbuildtestPipeline(
  script: this,
  service: 'worker-operator',
  dockerfile: 'test.Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"]
)
