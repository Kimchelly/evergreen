let $httpBackend;
let $httpParamSerializer;
let PERFORMANCE_ANALYSIS_AND_TRIAGE_API;
let API_V2;

describe('ExpandedMetricsSignalProcessingPage', () => {

  beforeEach(module('MCI'));

  describe('SignalProcessingCtrl', () => {
    let controller;
    let $scope;
    let CHANGE_POINTS_GRID;
    let PerformanceAnalysisAndTriageClient;
    let $routeParams;

    beforeEach(function () {
      module('MCI');
      inject(function (_$controller_, _PerformanceAnalysisAndTriageClient_, _$window_, _CHANGE_POINTS_GRID_, _$routeParams_, _$httpBackend_, _PERFORMANCE_ANALYSIS_AND_TRIAGE_API_, _CEDAR_APP_URL_, _API_V2_, _$httpParamSerializer_) {
        $scope = {};
        CHANGE_POINTS_GRID = _CHANGE_POINTS_GRID_;
        PerformanceAnalysisAndTriageClient = _PerformanceAnalysisAndTriageClient_;
        $routeParams = _$routeParams_;
        $routeParams.projectId = 'some-project';
        $httpBackend = _$httpBackend_;
        PERFORMANCE_ANALYSIS_AND_TRIAGE_API = _PERFORMANCE_ANALYSIS_AND_TRIAGE_API_;
        controller = _$controller_;
        API_V2 = _API_V2_;
        $httpParamSerializer = _$httpParamSerializer_
      })
    });

    function makeController() {
      controller('ExpandedMetricsSignalProcessingController', {
        '$scope': $scope,
        '$routeParams': $routeParams,
        'PerformanceAnalysisAndTriageClient': PerformanceAnalysisAndTriageClient,
        'CHANGE_POINTS_GRID': CHANGE_POINTS_GRID
      });
    }

    it('should authenticate on failed requests', () => {
      makeController();
      expectGetPageBase(0, 10, 511, $scope)
        .respond(function(){
          return [null, null, null, null, 'error']
        });
      $httpBackend.flush();
      $httpBackend.verifyNoOutstandingExpectation();
      $httpBackend.verifyNoOutstandingRequest();
      expect($scope.connectionError).toEqual(true);
    });

    it('should refresh the data when a new page is loaded', () => {
      makeController();
      expectGetPage(0, 10, 511, $scope);
      $scope.nextPage();
      expectGetPage(1, 10, 511, $scope, {
        'versions': Array(10).fill(
          {
            'version_id': 'another_version',
            'change_points': [
              {
                "_id": "another_id",
                "time_series_info": {
                  "project": "sys-perf",
                  "variant": "another_variant",
                  "task": "another_task",
                  "test": "another_test",
                  "measurement": "another_measurement",
                  "thread_level": 5
                },
                "cedar_perf_result_id": "7a8a54244e0bf868bf1d1edd2f388614aecb16bd",
                "version": "another_version",
                "order": 22151,
                "algorithm": {
                  "name": "e_divisive_means",
                  "version": 0,
                  "options": [
                    {
                      "name": "pvalue",
                      "value": 0.05
                    },
                    {
                      "name": "permutations",
                      "value": 100
                    }
                  ]
                },
                "triage": {
                  'triaged_on': '0001-01-01T00:00:00Z',
                  'triage_status': 'triaged'
                },
                "percent_change": 12.3248234827374,
                "calculated_on": "2020-05-04T20:21:12.037000"
              }]
          }),
        'page': 1,
        'page_size': 10,
        'total_pages': 511
      });
      expect($scope.page).toEqual(1);
      expect($scope.pageSize).toEqual(10);
      expect($scope.totalPages).toEqual(511);
      expect($scope.selection).toEqual([]);
      expect($scope.gridOptions.data).toEqual(Array(10).fill({
        id: 'another_id',
        version: 'another_version',
        variant: 'another_variant',
        task: 'another_task',
        test: 'another_test',
        measurement: 'another_measurement',
        percent_change: '12.32',
        triage_status: 'triaged',
        thread_level: 5,
        calculated_on: "2020-05-04T20:21:12.037000",
        revision: 'd00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0',
        build_id: 'some_project_another_variant_d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0_20_05_30_03_21_52',
        task_id: 'some_project_another_variant_another_task_d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0_20_05_30_03_21_52',
        revision_time: '2020-05-30T03:21:52Z',
      }));
      $httpBackend.verifyNoOutstandingExpectation();
      $httpBackend.verifyNoOutstandingRequest();
    });

    it('should be able to go to the previous page', () => {
      makeController();
      expectGetPage(0, 10, 511, $scope);
      $scope.page = 511;
      $scope.prevPage();
      expectGetPage(510, 10, 511, $scope);
      expect($scope.selection).toEqual([]);
      expect($scope.page).toEqual(510);
      expect($scope.pageSize).toEqual(10);
      expect($scope.totalPages).toEqual(511);
      expect($scope.gridOptions.data).toEqual(Array(10).fill({
        id: "test-id",
        version: 'sys_perf_085ffeb310e8fed49739cf8443fcb13ea795d867',
        variant: 'linux-standalone',
        task: 'large_scale_model',
        test: 'HotCollectionDeleter.Delete.2.2',
        measurement: 'AverageSize',
        percent_change: '50.32',
        triage_status: 'untriaged',
        thread_level: 0,
        calculated_on: "2020-05-04T20:21:12.037000",
        revision: 'd00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0',
        build_id: 'some_project_linux_standalone_d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0_20_05_30_03_21_52',
        task_id: 'some_project_linux_standalone_large_scale_model_d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0_20_05_30_03_21_52',
        revision_time: '2020-05-30T03:21:52Z',
      }));
      $httpBackend.verifyNoOutstandingExpectation();
      $httpBackend.verifyNoOutstandingRequest();
    });

    it('should be able to go to the next page', () => {
      makeController();
      expectGetPage(0, 10, 511, $scope);
      $scope.nextPage();
      expectGetPage(1, 10, 511, $scope);
      expect($scope.selection).toEqual([]);
      expect($scope.page).toEqual(1);
      expect($scope.pageSize).toEqual(10);
      expect($scope.totalPages).toEqual(511);
      expect($scope.gridOptions.data).toEqual(Array(10).fill({
        id: "test-id",
        version: 'sys_perf_085ffeb310e8fed49739cf8443fcb13ea795d867',
        variant: 'linux-standalone',
        task: 'large_scale_model',
        test: 'HotCollectionDeleter.Delete.2.2',
        measurement: 'AverageSize',
        percent_change: '50.32',
        triage_status: 'untriaged',
        thread_level: 0,
        calculated_on: "2020-05-04T20:21:12.037000",
        revision: 'd00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0',
        build_id: 'some_project_linux_standalone_d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0_20_05_30_03_21_52',
        task_id: 'some_project_linux_standalone_large_scale_model_d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0_20_05_30_03_21_52',
        revision_time: '2020-05-30T03:21:52Z',
      }));
      $httpBackend.verifyNoOutstandingExpectation();
      $httpBackend.verifyNoOutstandingRequest();
    });

    it('should get the first page on load', () => {
      makeController();
      expectGetPage(0, 10, 511, $scope);
      expect($scope.selection).toEqual([]);
      expect($scope.page).toEqual(0);
      expect($scope.pageSize).toEqual(10);
      expect($scope.totalPages).toEqual(511);
      expect($scope.gridOptions.data).toEqual(Array(10).fill({
        id: "test-id",
        version: 'sys_perf_085ffeb310e8fed49739cf8443fcb13ea795d867',
        variant: 'linux-standalone',
        task: 'large_scale_model',
        test: 'HotCollectionDeleter.Delete.2.2',
        measurement: 'AverageSize',
        percent_change: '50.32',
        triage_status: 'untriaged',
        thread_level: 0,
        calculated_on: "2020-05-04T20:21:12.037000",
        revision: 'd00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0',
        build_id: 'some_project_linux_standalone_d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0_20_05_30_03_21_52',
        task_id: 'some_project_linux_standalone_large_scale_model_d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0_20_05_30_03_21_52',
        revision_time: '2020-05-30T03:21:52Z',
      }));
      $httpBackend.verifyNoOutstandingExpectation();
      $httpBackend.verifyNoOutstandingRequest();
    });

    it('should set default pagination variables', () => {
      makeController();
      expect($scope.selection).toEqual([]);
      expect($scope.page).toEqual(0);
      expect($scope.pageSize).toEqual(10);
      expect($scope.totalPages).toEqual(1);
    });

    it('should apply filtering to server calls', () => {
      makeController();
      expectGetPage(0, 10, 511, $scope);
      $scope.variantRegex = 'some_variant';
      $scope.versionRegex = 'some_version';
      $scope.taskRegex = 'some_task';
      $scope.testRegex = 'some_test';
      $scope.measurementRegex = 'some_measurement';
      $scope.triageStatusRegex = "some_triage_status";
      $scope.threadLevels = [1,2,3];
      $scope.percentChangeWindows = ["0,20"];
      $scope.calculatedOnWindow = "2020-06-09T04:00:00.000,2020-06-18T04:00:00.000";
      $scope.nextPage();
      expectGetPage(1, 10, 511, $scope);
      $httpBackend.verifyNoOutstandingExpectation();
      $httpBackend.verifyNoOutstandingRequest();
    });

    it('should set up the grid, with defaults', () => {
      makeController();
      expect($scope.triageStatusRegex).toEqual('not_triaged');
      expect($scope.measurementRegex).toEqual('Latency50thPercentile|Latency95thPercentile');
      expect($scope.taskRegex).toEqual('mixed_writes_replica|large_scale_model|big_update|service_architecture_workloads|out_of_cache_scanner');
      expect($scope.testRegex).toEqual('^(?!.*(ActorFinished|ActorStarted|Setup|Cleanup))');
      delete $scope.gridOptions.onRegisterApi;
      for (definition of $scope.gridOptions.columnDefs) {
        delete definition["_link"];
      }
      expect($scope.gridOptions).toEqual({
        enableFiltering: true,
        enableRowSelection: true,
        useExternalSorting: true,
        enableSelectAll: true,
        enableSorting: true,
        selectionRowHeaderWidth: 35,
        useExternalFiltering: true,
        data: [],
        columnDefs: [
          {
            name: 'Percent Change',
            field: 'percent_change',
            cellTemplate: '<percent-change-cell row="row" ctx="grid.appScope.spvm.refCtx" />',
            filterHeaderTemplate: `
          <md-input-container style="margin:0">
            <md-select ng-model="col.filters[0].term" multiple>
              <md-option ng-value="hv" ng-repeat="hv in col.filters[0].hazardValues">{{hv}}</md-option>
            </md-select>
          </md-input-container>
        `,
            width: CHANGE_POINTS_GRID.HAZARD_COL_WIDTH,
            filter: {
              term: null,
              hazardValues: $scope.hazardValues
            },
          },
          {
            name: 'Revision',
            field: 'revision',
            type: 'string',
            enableFiltering: false
          },
          {
            name: 'Date',
            field: 'revision_time',
            type: 'string',
            enableFiltering: false
          },
          {
            name: 'Variant',
            field: 'variant',
            type: 'string',
            cellTemplate: 'ui-grid-link',
            enableSorting: false,
          },
          {
            name: 'Task',
            field: 'task',
            type: 'string',
            enableSorting: false,
            cellTemplate: 'ui-grid-link',
            filter: {
              term: 'mixed_writes_replica|large_scale_model|big_update|service_architecture_workloads|out_of_cache_scanner'
            }
          },
          {
            name: 'Test',
            field: 'test',
            type: 'string',
            enableSorting: false,
            cellTemplate: 'ui-grid-link',
            filter: {
              term: '^(?!.*(ActorFinished|ActorStarted|Setup|Cleanup))'
            }
          },
          {
            name: 'Version',
            field: 'version',
            type: 'string',
            cellTemplate: 'ui-grid-group-name',
            grouping: {
              groupPriority: 0,
            },
          },
          {
            name: 'Thread Level',
            field: 'thread_level',
            type: 'number',
            enableSorting: false,
          },
          {
            name: 'Measurement',
            field: 'measurement',
            type: 'string',
            filter: {
              term: 'Latency50thPercentile|Latency95thPercentile'
            },
            enableSorting: false,
          },
          {
            name: 'Triage Status',
            field: 'triage_status',
            type: 'string',
            filter: {
              term: 'not_triaged'
            },
            enableSorting: false,
          },
          {
            name: 'Calculated On',
            field: 'calculated_on',
            filterHeaderTemplate: '<md-date-range one-panel="true" auto-confirm="true" ng-model="selectedDate" md-on-select="col.filters[0].term = $dates"></md-date-range>',
            filter: {
              term: null
            },
            enableSorting: false,
          }
        ]
      });
    })
  });
});


function expectGetPage(page, pageSize, totalPages, $scope, newData) {
  let responseData = newData || standardData(page, pageSize, totalPages);
  expectGetPageBase(page, pageSize, totalPages, $scope, responseData);
  for (version of responseData.versions) {
    let versionUrl = `${API_V2.BASE}/${API_V2.VERSION_BY_ID}`.replace("{version_id}", version.version_id);
    $httpBackend.expectGET(versionUrl).respond(200, {
        "version_id": version.version_id,
        "create_time": "2020-05-30T03:21:52Z",
        "start_time": "2020-06-01T12:07:33.302Z",
        "revision": "d00b75bfcac3ac74036ac6c2ceec4e8b42ac93a0",
        "project": "sys-perf",
        "branch": "master",
      }
    );
  }
  $httpBackend.flush();
}

function expectGetPageBase( page, pageSize, totalPages, $scope, responseData) {
  let params = {
    page_size:pageSize,
    page: page,
  };
  if($scope.variantRegex) {
    params.variant_regex = $scope.variantRegex;
  }
  if($scope.versionRegex) {
    params.version_regex = $scope.versionRegex;
  }
  if($scope.taskRegex) {
    params.task_regex = $scope.taskRegex;
  }
  if($scope.testRegex) {
    params.test_regex = $scope.testRegex;
  }
  if($scope.measurementRegex) {
    params.measurement_regex = $scope.measurementRegex;
  }
  if($scope.triageStatusRegex) {
    params.triage_status_regex = $scope.triageStatusRegex;
  }
  if($scope.threadLevels) {
    params.thread_levels = [];
    $scope.threadLevels.forEach(tl => {
      params.thread_levels.push(tl);
    })
  }
  if($scope.percentChangeWindows) {
    params.percent_change = [];
    $scope.percentChangeWindows.forEach(w => {
      params.percent_change.push(w);
    })
  }
  if($scope.calculatedOnWindow) {
    params.calculated_on = $scope.calculatedOnWindow;
  }
  params = $httpParamSerializer(params);
  let url = `${PERFORMANCE_ANALYSIS_AND_TRIAGE_API.BASE + PERFORMANCE_ANALYSIS_AND_TRIAGE_API.CHANGE_POINTS_BY_VERSION.replace("{projectId}", "some-project")}?${params}`;

  return $httpBackend.expectGET(url).respond(200, JSON.stringify(responseData));
}

function standardData(page, pageSize, totalPages) {
  return {
    'versions': Array(10).fill(
      {
        'version_id': 'sys_perf_085ffeb310e8fed49739cf8443fcb13ea795d867',
        'change_points': [
          {
            "_id": "test-id",
            "time_series_info": {
              "project": "sys-perf",
              "variant": "linux-standalone",
              "task": "large_scale_model",
              "test": "HotCollectionDeleter.Delete.2.2",
              "measurement": "AverageSize",
              "thread_level": 0
            },
            "cedar_perf_result_id": "7a8a54244e0bf868bf1d1edd2f388614aecb16bd",
            "version": "sys_perf_085ffeb310e8fed49739cf8443fcb13ea795d867",
            "order": 22151,
            "algorithm": {
              "name": "e_divisive_means",
              "version": 0,
              "options": [
                {
                  "name": "pvalue",
                  "value": 0.05
                },
                {
                  "name": "permutations",
                  "value": 100
                }
              ]
            },
            "triage": {
              'triaged_on': '0001-01-01T00:00:00Z',
              'triage_status': 'untriaged'
            },
            "percent_change": 50.3248234827374,
            "calculated_on": "2020-05-04T20:21:12.037000"
          }],
      }),
    'page': page,
    'page_size': pageSize,
    'total_pages': totalPages
  }
}