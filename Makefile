target = waveshare-rp2040-zero
binary_name = firmware
src = microcontroller.go
# src = test.go
# src = scanner.go
dist_dir = dist
bootloader_drive = E:
baud_rate = 115200
serial_port = COM11

# ==================================================================================== #
# DIRECTORIES
# ==================================================================================== #

# Create the dist directory if it doesn't exist
$(dist_dir):
	mkdir -p $(dist_dir)

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## print this help message
.PHONY: help
help:
	@echo 'Usage:'; \
	awk '/^##/ { \
		sub(/^##\s*/, ""); \
		comment = $$0; \
		getline name; \
		split(name, a, " "); \
		print "    " a[2] ":" comment; \
	}' Makefile

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## run linting checks
.PHONY: lint
lint:
	gofmt -l . || true
	golangci-lint run ./...

# ==================================================================================== #
# BUILD AND FLASH
# ==================================================================================== #

## build the firmware
.PHONY: build
build: $(dist_dir)
	tinygo build -target=$(target) -o $(dist_dir)/$(binary_name).uf2 $(src)

## flash the firmware (manual copy to bootloader drive)
.PHONY: flash
flash: build
	@timeout=20; \
	echo -n "Waiting for bootloader drive $(bootloader_drive) to appear"; \
	while [ ! -d "$(bootloader_drive)" ] && [ $$timeout -gt 0 ]; do \
		echo -n ""; \
		sleep 1; \
		timeout=$$((timeout - 1)); \
	done; \
	echo ""; \
	if [ -d "$(bootloader_drive)" ]; then \
		echo "Copying firmware to bootloader drive $(bootloader_drive)"; \
		cp $(dist_dir)/$(binary_name).uf2 $(bootloader_drive)/; \
		echo "Firmware copied successfully"; \
	else \
		echo "Bootloader drive $(bootloader_drive) not found after waiting. Ensure the device is in bootloader mode"; \
		exit 1; \
	fi

## monitor the serial output with auto-reconnect
.PHONY: monitor
monitor:
	@echo "Monitoring serial output on port $(serial_port).."
	@while true; do \
		tinygo monitor -port=$(serial_port) -baudrate=$(baud_rate); \
		echo "Device disconnected. Retrying..."; \
		sleep 1; \
	done

## build, flash, and monitor in one step
.PHONY: run
run: flash monitor

## locally run the firmware
.PHONY: run/local
run/local:
	tinygo run $(src)

# ==================================================================================== #
# CLEANUP
# ==================================================================================== #

## remove generated files
.PHONY: clean
clean:
	rm -rf $(dist_dir)
