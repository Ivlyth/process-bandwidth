.PHONY: all | env

all: ebpf build
	@echo $(shell date)

.ONESHELL:
SHELL = /bin/sh

PARALLEL = $(shell $(CMD_GREP) -c ^processor /proc/cpuinfo)
MAKE = make

CMD_LLC ?= llc
CMD_TR ?= tr
CMD_CUT ?= cut
CMD_AWK ?= awk
CMD_SED ?= sed
CMD_GIT ?= git
CMD_CLANG ?= clang
CMD_STRIP ?= llvm-strip
CMD_RM ?= rm
CMD_INSTALL ?= install
CMD_MKDIR ?= mkdir
CMD_TOUCH ?= touch
CMD_PKGCONFIG ?= pkg-config
CMD_GO ?= go
CMD_GREP ?= grep
CMD_CAT ?= cat
CMD_MD5 ?= md5sum
CMD_BPFTOOL ?= bpftool
STYLE    ?= "{BasedOnStyle: Google, IndentWidth: 4}"

.check_%:
#
	@command -v $* >/dev/null
	if [ $$? -ne 0 ]; then
		echo "process-bandwidth Makefile: missing required tool $*"
		exit 1
	else
		touch $@ # avoid target rebuilds due to inexistent file
	fi

DEBUG_PRINT ?=
ifeq ($(DEBUG),1)
DEBUG_PRINT := -DDEBUG_PRINT
endif

BPFHEADER = -I./tracepoint \

EXTRA_CFLAGS ?= -emit-llvm -O2 -S\
	-xc -g \
	-D__BPF_TRACING__ \
	-D__KERNEL__ \
	$(DEBUG_PRINT) \
	-Wall \
	-Wno-unused-variable \
	-Wno-frame-address \
	-Wno-unused-value \
	-Wno-unknown-warning-option \
	-Wno-pragma-once-outside-header \
	-Wno-pointer-sign \
	-Wno-gnu-variable-sized-type-not-at-end \
	-Wno-deprecated-declarations \
	-Wno-compare-distinct-pointer-types \
	-Wno-address-of-packed-member \
	-fno-stack-protector \
	-fno-jump-tables \
	-fno-unwind-tables \
	-fno-asynchronous-unwind-tables
#
# tools version
#

CLANG_VERSION = $(shell $(CMD_CLANG) --version 2>/dev/null | \
	head -1 | $(CMD_TR) -d '[:alpha:]' | $(CMD_TR) -d '[:space:]' | $(CMD_CUT) -d'.' -f1)

#  clang 编译器版本检测，llvm检测，
.checkver_$(CMD_CLANG): \
	| .check_$(CMD_CLANG)
#
	@echo $(shell date)
	@if [ ${CLANG_VERSION} -lt 9 ]; then
		echo -n "you MUST use clang 9 or newer, "
		echo "your current clang version is ${CLANG_VERSION}"
		exit 1
	fi
	$(CMD_TOUCH) $@ # avoid target rebuilds over and over due to inexistent file

GO_VERSION = $(shell $(CMD_GO) version 2>/dev/null | $(CMD_AWK) '{print $$3}' | $(CMD_SED) 's:go::g' | $(CMD_CUT) -d. -f1,2)
GO_VERSION_MAJ = $(shell echo $(GO_VERSION) | $(CMD_CUT) -d'.' -f1)
GO_VERSION_MIN = $(shell echo $(GO_VERSION) | $(CMD_CUT) -d'.' -f2)


# golang 版本检测  1.16 以上
.checkver_$(CMD_GO): \
	| .check_$(CMD_GO)
#
	@if [ ${GO_VERSION_MAJ} -eq 1 ]; then
		if [ ${GO_VERSION_MIN} -lt 16 ]; then
			echo -n "you MUST use golang 1.16 or newer, "
			echo "your current golang version is ${GO_VERSION}"
			exit 1
		fi
	fi
	touch $@

# bpftool version
.checkver_$(CMD_BPFTOOL): \
	| .check_$(CMD_BPFTOOL)

#
# version
#
# tags date info
TAG_COMMIT := $(shell git rev-list --abbrev-commit --tags --max-count=1)
TAG := $(shell git describe --abbrev=0 --tags ${TAG_COMMIT} 2>/dev/null || true)
COMMIT := $(shell git rev-parse --short HEAD)
# DATE := $(shell #git log -1 --format=%cd --date=format:"%Y%m%d")
DATE := 20220504
LAST_GIT_TAG := $(TAG:v%=%)-$(DATE)-$(COMMIT)
ifneq ($(COMMIT), $(TAG_COMMIT))
	LAST_GIT_TAG := $(LAST_GIT_TAG)-prev-$(TAG_COMMIT)
endif

ifneq ($(shell git status --porcelain),)
	LAST_GIT_TAG := $(LAST_GIT_TAG)-dirty
endif

#LAST_GIT_TAG ?= $(shell $(CMD_GIT) describe --tags --match 'v*' 2>/dev/null)
VERSION ?= $(if $(RELEASE_TAG),$(RELEASE_TAG),$(LAST_GIT_TAG))

#
# environment
#

UNAME_M := $(shell uname -m)
UNAME_R := $(shell uname -r)


#
# Kernel Version detect
#

KERNEL_LESS_5_2_FLAGS ?=
KERNEL_LESS_VERSION := 5.2.0
HIGHER_VERSION := $(shell echo -e "$(UNAME_R)\n$(KERNEL_LESS_VERSION)" | sort -V | tail --lines=1)

# kernel version compare
ifeq ($(HIGHER_VERSION),$(KERNEL_LESS_VERSION))
   # its an older kernel
   KERNEL_LESS_5_2_FLAGS = -DKERNEL_LESS_5_2
else
  # its a newer kernel
endif


#
# Target Arch
#

ifeq ($(UNAME_M),x86_64)
   ARCH = x86_64
   LINUX_ARCH = x86
   GO_ARCH = amd64
endif

ifeq ($(UNAME_M),aarch64)
   ARCH = arm64
   LINUX_ARCH = arm64
   GO_ARCH = arm64
endif

#
# include vpath
#

KERN_RELEASE ?= $(UNAME_R)
KERN_BUILD_PATH ?= $(if $(KERN_HEADERS),$(KERN_HEADERS),/lib/modules/$(KERN_RELEASE)/build)
KERN_SRC_PATH ?= $(if $(KERN_HEADERS),$(KERN_HEADERS),$(if $(wildcard /lib/modules/$(KERN_RELEASE)/source),/lib/modules/$(KERN_RELEASE)/source,$(KERN_BUILD_PATH)))

BPF_TAG = $(subst .,_,$(KERN_RELEASE)).$(subst .,_,$(VERSION))


#
# BPF Source file
#

TARGETS := tracepoint/tracepoint

# Generate file name-scheme based on TARGETS
KERN_SOURCES = ${TARGETS:=.c}
KERN_OBJECTS = ${KERN_SOURCES:.c=.o}

.PHONY: env
env:
	@echo ---------------------------------------
	@echo "Makefile Environment:"
	@echo ---------------------------------------
	@echo "PARALLEL                 $(PARALLEL)"
	@echo ---------------------------------------
	@echo "CLANG_VERSION            $(CLANG_VERSION)"
	@echo "GO_VERSION               $(GO_VERSION)"
	@echo ---------------------------------------
	@echo "CMD_CLANG                $(CMD_CLANG)"
	@echo "CMD_GIT                  $(CMD_GIT)"
	@echo "CMD_GO                   $(CMD_GO)"
	@echo "CMD_INSTALL              $(CMD_INSTALL)"
	@echo "CMD_LLC                  $(CMD_LLC)"
	@echo "CMD_MD5                  $(CMD_MD5)"
	@echo "CMD_PKGCONFIG            $(CMD_PKGCONFIG)"
	@echo "CMD_STRIP                $(CMD_STRIP)"
	@echo "VERSION                  $(VERSION)"
	@echo "LAST_GIT_TAG             $(LAST_GIT_TAG)"
	@echo "TAG_COMMIT               $(TAG_COMMIT)"
	@echo "TAG                      $(TAG)"
	@echo "COMMIT                   $(COMMIT)"
	@echo "BPF_TAG                  $(BPF_TAG)"
	@echo ---------------------------------------
	@echo "UNAME_M                  $(UNAME_M)"
	@echo "UNAME_R                  $(UNAME_R)"
	@echo "ARCH                     $(ARCH)"
	@echo "LINUX_ARCH               $(LINUX_ARCH)"
	@echo ---------------------------------------
	@echo "KERN_RELEASE             $(KERN_RELEASE)"
	@echo "KERN_BUILD_PATH          $(KERN_BUILD_PATH)"
	@echo "KERN_SRC_PATH            $(KERN_SRC_PATH)"
	@echo ---------------------------------------
	@echo "GO_ARCH                  $(GO_ARCH)"
	@echo ---------------------------------------

#
# usage
#

.PHONY: help
help:
	@echo "# environment"
	@echo "    $$ make env		# show makefile environment/variables"
	@echo ""
	@echo "# build"
	@echo "    $$ make all		# build process-bandwidth"
	@echo ""
	@echo "# clean"
	@echo "    $$ make clean		# wipe ./bin/ ./user/bytecode/ "
	@echo ""
	@echo "# test"
	@echo "    $$ make test		# run all go"

.PHONY: clean build ebpf

.PHONY: clean
clean:
	$(CMD_RM) -f engine/*.o
	$(CMD_RM) -f bin/process-bandwidth
	$(CMD_RM) -f .check*

.PHONY: build
build: \
	.checkver_$(CMD_GO)
	GOOS=linux CGO_ENABLED=0 sh build.sh

.PHONY: rsync
rsync: RHOST ?= 10.0.81.8
rsync: build
	/Users/myth/.PyEnvs/ambot/bin/ambot-run.py -t ${RHOST} @cp bin/process-bandwidth /root/

.PHONY: ebpf
ebpf: $(KERN_OBJECTS)

.PHONY: $(KERN_OBJECTS)
$(KERN_OBJECTS): %.o: %.c \
	| .checkver_$(CMD_CLANG) \
	.checkver_$(CMD_GO)
	$(CMD_CLANG) \
    		$(BPFHEADER) \
    		-I $(KERN_SRC_PATH)/arch/$(LINUX_ARCH)/include \
    		-I $(KERN_SRC_PATH)/arch/$(LINUX_ARCH)/include/uapi \
    		-I $(KERN_BUILD_PATH)/arch/$(LINUX_ARCH)/include/generated \
    		-I $(KERN_BUILD_PATH)/arch/$(LINUX_ARCH)/include/generated/uapi \
    		-I $(KERN_SRC_PATH)/include \
    		-I $(KERN_BUILD_PATH)/include \
    		-I $(KERN_SRC_PATH)/include/uapi \
    		-I $(KERN_BUILD_PATH)/include/generated \
    		-I $(KERN_BUILD_PATH)/include/generated/uapi \
    		$(EXTRA_CFLAGS) \
    		$(KERNEL_LESS_5_2_FLAGS) \
    		-c $< \
    		-o - |$(CMD_LLC) \
    		-march=bpf \
    		-filetype=obj \
    		-o $(subst tracepoint/,engine/,$(subst .c,.o,$<))

# Format the code
format:
	@echo "  ->  Formatting code"
	@clang-format -i -style=$(STYLE) tracepoint/*.c
	@clang-format -i -style=$(STYLE) tracepoint/common.h