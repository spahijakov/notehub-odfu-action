# Notehub Outboard Firmware Deployment GitHub Action

A Go-based GitHub Action for deploying [outboard firmware](https://dev.blues.io/notehub/host-firmware-updates/notecard-outboard-firmware-update/#notecard-outboard-firmware-update) updates to devices via the Notehub API. This action handles OAuth2 authentication, firmware upload, and outboard device firmware update (ODFU) triggering.

> [!WARNING]
> This action is experimental and support is not guaranteed at this time. This is subject to change.

## Features

- **OAuth2 Authentication**: Secure authentication with Notehub API on a per-project basis
- **Binary Firmware Upload**: Direct upload of outboard firmware binaries to Notehub
- **Device Targeting**: Multiple targeting options (device UID, tags, serial numbers, fleets, etc.)
- **Docker-based**: Lightweight, containerized execution

## Supported Host MCUs

- STM32 ([Swan, Cygnet](https://blues.com/feather-mcu/))
- ESP32
- Any other MCU that supports MCUBoot (e.g. nRF52)

## Setup

### 1. Create Notehub Project

1. Log into your [Notehub](https://notehub.io) account
2. Create and/or navigate to your target project's settings
3. Create a new `Programmatic API access` Client ID and Client Secret
4. Copy the generated `Client ID` and `Client Secret` to your clipboard

### 2. Configure GitHub Secrets

1. Go to your GitHub repository
2. Navigate to **Settings** → **Secrets and variables** → **Actions**
3. Create two repository secrets:

   **First Secret:**
   - Name: `NOTEHUB_CLIENT_ID`
   - Value: Your Notehub Programmatic API access Client ID

   **Second Secret:**
   - Name: `NOTEHUB_CLIENT_SECRET`
   - Value: Your Notehub Programmatic API access Client Secret

## Usage

### Basic Example

Create a workflow file (e.g., `.github/workflows/deploy-firmware.yml`):

```yaml
name: Deploy Firmware to Notehub

on:
  push:
    tags:
      - 'v*'  # Trigger on version tags
  workflow_dispatch:
    inputs:
      device_uid:
        description: 'Target Device UID (optional)'
        required: false
        type: string

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Build firmware
        run: |
          # Your firmware build steps here
          mkdir -p build
          echo "Mock firmware binary" > build/firmware.bin

      - name: Deploy to Notehub
        uses: docker://Bucknalla/notehub-dfu-github:latest
        with:
          issue_dfu: 'true'
          project_uid: 'app:12345678-1234-1234-1234-123456789abc'
          firmware_file: 'build/firmware.bin'
          device_uid: ${{ inputs.device_uid }}
          client_id: ${{ secrets.NOTEHUB_CLIENT_ID }}
          client_secret: ${{ secrets.NOTEHUB_CLIENT_SECRET }}
```

### Advanced Example with Device Targeting

```yaml
name: Deploy Release to Production Fleet

on:
  release:
    types: [published]

jobs:
  deploy-to-production:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Download firmware artifact
        uses: actions/download-artifact@v4
        with:
          name: firmware-build
          path: ./firmware

      - name: Deploy to production devices
        uses: docker://Bucknalla/notehub-dfu-github:latest
        with:
          # Required parameters
          project_uid: ${{ vars.NOTEHUB_PROJECT_UID }}
          firmware_file: 'firmware/app-v1.2.3.bin'
          client_id: ${{ secrets.NOTEHUB_CLIENT_ID }}
          client_secret: ${{ secrets.NOTEHUB_CLIENT_SECRET }}

          # Target production devices by tag
          tag: 'production'
          fleet_uid: ${{ vars.PRODUCTION_FLEET_UID }}

          # Optional parameters
          notecard_firmware: '8.1.4'
          location: 'London'
          sku: 'NOTE-WBNAW'

  deploy-to-specific-device:
    runs-on: ubuntu-latest
    if: github.event.inputs.device_uid != ''
    steps:
      - uses: actions/checkout@v4

      - name: Deploy to specific devices
        uses: docker://Bucknalla/notehub-dfu-github:latest
        with:
          issue_dfu: 'true'
          project_uid: ${{ vars.NOTEHUB_PROJECT_UID }}
          firmware_file: 'build/firmware.bin'
          client_id: ${{ secrets.NOTEHUB_CLIENT_ID }}
          client_secret: ${{ secrets.NOTEHUB_CLIENT_SECRET }}
          device_uid: dev:12345678,dev:12345679,dev:12345680
```

## Action Inputs

### Required Inputs

| Input           | Description                                   | Example                                    |
| --------------- | --------------------------------------------- | ------------------------------------------ |
| `project_uid`   | Notehub Project UID                           | `app:12345678-1234-1234-1234-123456789abc` |
| `firmware_file` | Path to firmware file (relative to repo root) | `build/firmware.bin`                       |
| `client_id`     | Notehub OAuth2 Client ID                      | `${{ secrets.NOTEHUB_CLIENT_ID }}`         |
| `client_secret` | Notehub OAuth2 Client Secret                  | `${{ secrets.NOTEHUB_CLIENT_SECRET }}`     |

### Optional Device Targeting

All of the following inputs are optional and can be used together. Multiple values can be provided by separating them with a comma, e.g. `tag1,tag2,tag3`.

| Input               | Description                      | Example                      |
| ------------------- | -------------------------------- | ---------------------------- |
| `device_uid`        | Target specific device by UID    | `dev:12345678`               |
| `tag`               | Target devices with specific tag | `production`                 |
| `serial_number`     | Target device by serial number   | `SN123456`                   |
| `fleet_uid`         | Target devices in specific fleet | `fleet:abcdef`               |
| `product_uid`       | Specify product UID              | `com.company.product:sensor` |
| `notecard_firmware` | Notecard firmware version        | `8.1.4`                      |
| `location`          | Device location                  | `London`                     |
| `sku`               | Notecard SKU                     | `NOTE-WBNAW`          |

## Action Outputs

| Output              | Description                        |
| ------------------- | ---------------------------------- |
| `deployment_status` | Status of the firmware deployment  |
| `firmware_filename` | Name of the uploaded firmware file |

## Example Workflow

The [build-and-deploy.yml](.github/workflows/build-and-deploy.yml) workflow is a example of how to use this action with the Swan and Cygnet firmware. It builds the firmware using the Arduino CLI, uploads it to Notehub, and triggers an ODFU on the specified devices.
