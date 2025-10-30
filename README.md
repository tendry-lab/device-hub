[![Device-Hub](https://github.com/tendry-lab/device-hub/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/tendry-lab/device-hub/actions/workflows/build.yml)

## Introduction

device-hub is a self-hosted software solution for collecting, storing, and monitoring data from IoT devices on a local network. It was originally designed to work with devices based on the [control-components](https://github.com/tendry-lab/control-components) firmware, but it actually supports any device with an HTTP API, making it suitable for a wide range of smart home, industrial automation, and IoT data monitoring applications.

## How It Works

First, an IoT device should be manually or automatically registered in the device-hub. Then, device-hub starts to fetch device telemetry and registration data and stores it in the long-term storage. In addition, device-hub stores information about registered devices, so when it's restarted it automatically reconnects to registered devices, and much more, see the full [list](#Features) of supported features.

## Features

- [Device Data Storage](docs/features.md#Device-Data-Storage)
- [System Time Synchronization](docs/features.md#System-Time-Synchronization)
- [Inactive Device Monitoring](docs/features.md#Inactive-Device-Monitoring)
- [mDNS Server](docs/features.md#mDNS-Server)
- [mDNS Browser](docs/features.md#mDNS-Browser)
- [mDNS Auto Discovery](docs/features.md#mDNS-Auto-Discovery)

## Contribution

- [Semver](https://semver.org/) is used for versioning.
- Try to keep PR small.
- New code should be similar to existing code. Use the [Google Go Style Guide](https://google.github.io/styleguide/go/).

## License

This project is licensed under the Apache 2.0 License - see the LICENSE file for details.
