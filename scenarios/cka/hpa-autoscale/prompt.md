In namespace `autoscale`, create a HorizontalPodAutoscaler `php-apache` for
deployment `php-apache` that keeps average CPU utilisation at 50%, with
between 1 and 4 replicas. When load drops it must wait 30 seconds before
scaling down.

The autoscaler must be able to compute its metric. metrics-server is already
installed.
