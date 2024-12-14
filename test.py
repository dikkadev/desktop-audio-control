import hid

VID = 0x1234
PID = 0x5678

with hid.Device(VID, PID) as device:
    device.set_nonblocking(True)

    while True:
        data = device.read(3)  # Read 3 bytes
        if data:
            print(f"Received: {data}")
