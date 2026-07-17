// Properties starting with _ are optional

test {
    activated       = true
    dirBase         = "files/tests"
    files           = ["test.csv"]
    maxLinesPerFile = 25
}

source {
    files           = ["agendamentos.csv"]
    dirBase         = "files/source"
    separator      = ","
}

target {
    dirBase         = "files/target"
    maxLinesPerFile = 100000
    _separator       = ","
    // _sqlTable        = "dbname.table_name"
}

filter {
    _columns         = ["protocolo", "competAgendamento"]
    _field "competAgendamento" {
        filter {
            cond    = ">=" // <, <=, !=, >, >=
            val     = 202608
        }
    }
}
