#!/usr/bin/env bash
# Single source of the OMG SysML v2 Pilot Implementation pin, sourced by every
# script that fetches something from it: the training corpus, the additional OMG
# corpora, and the reference validator the differential harness compares against.
#
# Kept in one file so the release under comparison cannot drift between them.
PILOT_TAG="${PILOT_TAG:-2026-05}"
PILOT_REPO="${PILOT_REPO:-https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation.git}"
PILOT_ARTIFACT_VERSION="${PILOT_ARTIFACT_VERSION:-0.60.1}"
