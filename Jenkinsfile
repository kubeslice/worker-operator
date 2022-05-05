@Library('jenkins-library@opensource') _
dockerImagePipeline1(
  script: this,
  service: 'aveshadev/worker-operator',
  dockerfile: 'Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"],
  trigger_remote: 'yes'
)
