# Aurora
[![Build Status](https://circleci.com/gh/hcnet/go.svg?style=shield)](https://circleci.com/gh/hcnet/go)

Aurora is the client facing API server for the [Hcnet ecosystem](https://developers.hcnet.org/docs/start/introduction/).  It acts as the interface between [Hcnet Core](https://developers.hcnet.org/docs/run-core-node/) and applications that want to access the Hcnet network. It allows you to submit transactions to the network, check the status of accounts, subscribe to event streams and more.

Check out the following resources to get started:
- [Aurora Development Guide](internal/docs/GUIDE_FOR_DEVELOPERS.md): Instructions for building and developing Aurora. Covers setup, building, testing, and contributing. Also contains some helpful notes and context for Aurora developers.
- [Quickstart Guide](https://github.com/HashCash-Consultants/quickstart): An external tool provided from a separate repository. It builds a docker image which can be used for running the hcnet stack including Aurora locally for evaluation and testing situations. A great way to observe a reference runtime deployment, to see how everything fits together.
- [Aurora Testing Guide](internal/docs/TESTING_NOTES.md): Details on how to test Aurora, including unit tests, integration tests, and end-to-end tests.
- [Aurora SDK and API Guide](internal/docs/SDK_API_GUIDE.md): Documentation on the Aurora SDKs, APIs, resources, and examples. Useful for developers building on top of Aurora.

## Run a production server
If you're an administrator planning to run a production instance of Aurora as part of the public Hcnet network, you should check out the instructions on our public developer docs - [Run an API Server](https://developers.hcnet.org/docs/run-api-server/). It covers installation, monitoring, error scenarios and more.

## Contributing
As an open source project, development of Aurora is public, and you can help! We welcome new issue reports, documentation and bug fixes, and contributions that further the project roadmap. The [Development Guide](internal/docs/GUIDE_FOR_DEVELOPERS.md) will show you how to build Aurora, see what's going on behind the scenes, and set up an effective develop-test-push cycle so that you can get your work incorporated quickly.
