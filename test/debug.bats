setup() {
    load 'test_helper/bats-support/load'
    load 'test_helper/bats-assert/load'

    DIR="$( cd "$( dirname "$BATS_TEST_FILENAME" )" >/dev/null 2>&1 && pwd )"
    PATH="$DIR/../bin:$PATH"

    docker run --ipc 'shareable' --name debug_me --rm -d alpine tail -f /dev/null
}

teardown() {
    docker rm -f debug_me
}

function debug_command_with_default_image_works { # @test
    run slim debug debug_me -- ps

    assert [ $status -eq 0 ]

    assert_output --partial '1 root      0:00 tail -f /dev/null'
}

function debug_command_with_custom_image_works { # @test
    run slim debug --debug-image busybox debug_me -- cat /proc/1/root/etc/os-release

    assert [ $status -eq 0 ]

    assert_output --partial 'NAME="Alpine Linux"'
    assert_output --partial 'ID=alpine'
}
