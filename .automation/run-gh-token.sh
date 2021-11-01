#!/bin/bash -x

################################################################################
############################## RUN GH-TOKEN ####################################
################################################################################

###########
# Globals #
###########
ACTION="${ACTION:-generate}"                         # Action to perform (generate,revoke,installations)
PRIVATE_KEY="${PRIVATE_KEY:-null}"                   # PRIVATE_KEY from users created GitHub App
APP_ID="${APP_ID:-null}"                             # APP_ID from users created GitHub App
TOKEN="${TOKEN:-null}"                               # The generated GitHub Personal Access Token
DURATION="${DURATION:-10}"                           # Duration of the Tokens Life Default 10 min
GITHUB_HOSTNAME="${GITHUB_HOSTNAME:-api.github.com}" # Hostname to call API endpoints
INSTALLATION_ID="${INSTALLATION_ID:-null}"           # INSTALLATION_ID from users created GitHub App
NUM_REGEX='^[0-9]+$'                                 # Regex to check numbers

################################################################################
############################## Functions Below #################################
################################################################################
################################################################################
#### Function ValidateInput ####################################################
ValidateInput() {
  #######################################
  # Validate what action we are running #
  #######################################
  # Make lowercase
  ACTION=$(echo "${ACTION}"| awk '{print tolower($0)}')
  if [ "${ACTION}" == "generate" ] || [ "${ACTION}" == "installations" ]; then
    # Validate we have a PRIVATE_KEY
    if [ "${PRIVATE_KEY}" == 'null' ]; then
      echo "ERROR! [PRIVATE_KEY] was not set!"
      echo "You must either pass the key itself, or a path to the [PRIVATE_KEY]!"
      exit 1
    fi
    # Validate we have an APP_ID
    if [ "${APP_ID}" == 'null' ] || ! [[ ${APP_ID} =~ ${NUM_REGEX} ]]; then
      echo "ERROR! [APP_ID] was not set, or is not a number!"
      echo "You must pass the [APP_ID] to generate a token or check installations!"
      exit 1
    fi
    # Validate we have an DURATION
    if [ "${DURATION}" == 'null' ] || ! [[ ${DURATION} =~ ${NUM_REGEX} ]]; then
      echo "ERROR! [DURATION] was not set, or is not a number!"
      echo "You must pass a valid [DURATION] in minutes to generate a token or check installations!"
      exit 1
    fi
  elif [ "${ACTION}" == "revoke" ]; then
    # Validate we have an TOKEN
    if [ "${TOKEN}" == 'null' ] || [ ${#TOKEN} -ne 40 ]; then
      echo "ERROR! [TOKEN] was not set, or is not the correct size!"
      echo "You must pass a valid [TOKEN] to revoke!"
      exit 1
    fi
  else
    # Got an ACTINO that is not a fit
    echo "ERROR! ACTION needs to be 'generate', 'revoke', or 'installations'!"
    echo "Recieved:[${ACTION}]"
    exit 1
  fi
}
################################################################################
#### Function RunAction ########################################################
RunAction() {
  ####################
  # Generate a token #
  ####################
  if [ "${ACTION}" == "generate" ]; then
    # Build the basic command
    COMMAND="./gh-token generate --key ${PRIVATE_KEY} --app_id ${APP_ID} --duration ${DURATION} --hostname ${GITHUB_HOSTNAME}"
    # Add the INSTALLATION_ID if set
    if [[ ${INSTALLATION_ID} =~ ${NUM_REGEX} ]]; then
      COMMAND+=" --installation_id ${INSTALLATION_ID}"
    fi
    # Run the generate command
    eval "${COMMAND}"
    # will need to catch the return and parse the token
    # echo "BUILD_DATE=\"$(date -u +'%Y-%m-%dT%H:%M:%SZ')\" >> ${GITHUB_ENV}"
  ##################
  # Revoke a token #
  ##################
  elif [ "${ACTION}" == "revoke" ]; then
    # Build the basic command
    COMMAND="./gh-token revoke --token ${TOKEN} --hostname ${GITHUB_HOSTNAME}"
    # Run the generate command
    eval "${COMMAND}"
  ###########################
  # Check the installations #
  ###########################
  elif [ "${ACTION}" == "installations" ]; then
    # Build the basic command
    COMMAND="./gh-token installations --key ${PRIVATE_KEY} --app_id ${APP_ID} --duration ${DURATION} --hostname ${GITHUB_HOSTNAME}"
    # Run the generate command
    eval "${COMMAND}"
  fi
}
################################################################################
################################### MAIN #######################################
################################################################################

##################
# Validate Input #
##################
ValidateInput

##############
# Run Action #
##############
RunAction
